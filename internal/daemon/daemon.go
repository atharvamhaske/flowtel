package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMaxLineBytes = 64 * 1024 * 1024
	requestParseError   = -32700
	invalidRequest      = -32600
	methodNotFound      = -32601
	invalidParams       = -32602
	internalError       = -32603
	applicationError    = -32000
)

type Config struct {
	SocketPath    string
	DataDir       string
	DaemonVersion string
	Sources       []string
	MaxLineBytes  int
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.SocketPath) == "" || strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("daemon paths are required")
	}
	if c.MaxLineBytes < 1 {
		return fmt.Errorf("daemon max line bytes must be positive")
	}
	return nil
}

func DefaultConfig() (Config, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if runtimeDir == "" {
		runtimeDir = filepath.Join(home, ".flowtel", "run")
	} else {
		runtimeDir = filepath.Join(runtimeDir, "flowtel")
	}
	dataDir := os.Getenv("FLOWTEL_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".flowtel", "data")
	}
	socketPath := os.Getenv("FLOWTEL_DAEMON_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(runtimeDir, "daemon.sock")
	}
	return Config{
		SocketPath: socketPath, DataDir: dataDir, DaemonVersion: "dev",
		Sources: []string{"pi", "claude-code", "opencode", "debug"}, MaxLineBytes: DefaultMaxLineBytes,
	}, nil
}

type Sink interface {
	Accept(context.Context, Envelope) error
}

type NopSink struct{}

func (NopSink) Accept(context.Context, Envelope) error { return nil }

type Daemon struct {
	config    Config
	sink      Sink
	listener  net.Listener
	started   time.Time
	journal   *os.File
	journalMu sync.Mutex
	stateMu   sync.Mutex
	sessions  map[string]SessionStatus
	stored    atomic.Int64
	closed    atomic.Bool
}

func New(config Config, sink Sink) (*Daemon, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate daemon config: %w", err)
	}
	if sink == nil {
		sink = NopSink{}
	}
	if err := os.MkdirAll(config.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create daemon data directory: %w", err)
	}
	journal, err := os.OpenFile(filepath.Join(config.DataDir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open daemon journal: %w", err)
	}
	return &Daemon{config: config, sink: sink, journal: journal, sessions: make(map[string]SessionStatus)}, nil
}

func (d *Daemon) Serve(ctx context.Context) error {
	if d == nil {
		return fmt.Errorf("serve daemon: daemon is nil")
	}
	if err := os.MkdirAll(filepath.Dir(d.config.SocketPath), 0o700); err != nil {
		return fmt.Errorf("create daemon socket directory: %w", err)
	}
	if err := removeSocket(d.config.SocketPath); err != nil {
		return err
	}
	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on daemon socket: %w", err)
	}
	d.listener = listener
	d.started = time.Now()
	defer d.close()
	go func() {
		<-ctx.Done()
		_ = d.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if d.closed.Load() || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept daemon connection: %w", err)
		}
		go d.ServeConn(ctx, connection)
	}
}

// ServeConn serves one already-accepted protocol connection. It is useful for
// embedders and tests that provide their own transport.
func (d *Daemon) ServeConn(ctx context.Context, connection net.Conn) {
	d.handleConnection(ctx, connection)
}

func (d *Daemon) Close() error {
	if d == nil || !d.closed.CompareAndSwap(false, true) {
		return nil
	}
	if d.listener != nil {
		_ = d.listener.Close()
	}
	if d.journal != nil {
		if err := d.journal.Close(); err != nil {
			return fmt.Errorf("close daemon journal: %w", err)
		}
	}
	return nil
}

func (d *Daemon) close() {
	_ = d.Close()
	_ = os.Remove(d.config.SocketPath)
}

func (d *Daemon) handleConnection(ctx context.Context, connection net.Conn) {
	defer func() { _ = connection.Close() }()
	scanner := bufio.NewScanner(connection)
	scanner.Buffer(make([]byte, 4096), d.config.MaxLineBytes)
	initialized := false
	for scanner.Scan() {
		request, err := decodeRequest(scanner.Bytes())
		if err != nil {
			_ = writeResponse(connection, Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: rpcError(requestParseError, err.Error())})
			continue
		}
		if !initialized && request.Method != "initialize" {
			_ = writeResponse(connection, Response{JSONRPC: "2.0", ID: request.ID, Error: rpcError(invalidRequest, "initialize is required")})
			continue
		}
		response, respond := d.dispatch(ctx, request)
		if request.Method == "initialize" && response.Error == nil {
			initialized = true
		}
		if respond {
			if err := writeResponse(connection, response); err != nil {
				return
			}
		}
		if request.Method == "daemon.shutdown" && response.Error == nil {
			return
		}
	}
}

func (d *Daemon) dispatch(ctx context.Context, request Request) (Response, bool) {
	response := Response{JSONRPC: "2.0", ID: request.ID}
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion int    `json:"protocol_version"`
			Client          Client `json:"client"`
		}
		if err := decodeParams(request.Params, &params); err != nil || params.ProtocolVersion != ProtocolVersion {
			response.Error = rpcError(applicationError, "unsupported protocol version")
			return response, true
		}
		response.Result = map[string]any{"protocol_version": ProtocolVersion, "daemon_version": d.config.DaemonVersion, "capabilities": map[string]any{"sources": d.config.Sources}}
	case "event.log":
		var envelope Envelope
		if err := decodeParams(request.Params, &envelope); err != nil {
			response.Error = rpcError(invalidParams, err.Error())
			return response, true
		}
		if err := envelope.Validate(); err != nil {
			response.Error = rpcError(invalidParams, err.Error())
			return response, true
		}
		if err := d.append(ctx, envelope); err != nil {
			response.Error = rpcError(internalError, err.Error())
			return response, true
		}
		response.Result = map[string]any{"accepted": true}
	case "session.flush":
		var params struct {
			SessionID string `json:"session_id"`
			TimeoutMS int    `json:"timeout_ms"`
		}
		if err := decodeParams(request.Params, &params); err != nil || params.SessionID == "" {
			response.Error = rpcError(invalidParams, "session_id is required")
			return response, true
		}
		response.Result = map[string]any{"flushed": true, "pending": 0, "accepted_sessions": d.sessionCount(params.SessionID)}
	case "status.get":
		response.Result = d.status()
	case "daemon.shutdown":
		response.Result = map[string]any{"ok": true}
		go func() { _ = d.Close() }()
	default:
		response.Error = rpcError(methodNotFound, "method not found")
	}
	if request.ID == nil || string(request.ID) == "" {
		return response, false
	}
	return response, true
}

func (d *Daemon) append(ctx context.Context, envelope Envelope) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("append event: %w", ctx.Err())
	default:
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	d.journalMu.Lock()
	_, writeErr := d.journal.Write(append(encoded, '\n'))
	if writeErr == nil {
		writeErr = d.journal.Sync()
	}
	d.journalMu.Unlock()
	if writeErr != nil {
		return fmt.Errorf("journal event: %w", writeErr)
	}
	d.stateMu.Lock()
	session := d.sessions[envelope.SessionID]
	session.SessionID = envelope.SessionID
	session.Source = envelope.Source
	d.sessions[envelope.SessionID] = session
	d.stateMu.Unlock()
	d.stored.Add(1)
	if err := d.sink.Accept(ctx, envelope); err != nil {
		return fmt.Errorf("accept event in sink: %w", err)
	}
	return nil
}

func (d *Daemon) sessionCount(sessionID string) int {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	if _, ok := d.sessions[sessionID]; ok {
		return 1
	}
	return 0
}

func (d *Daemon) status() Status {
	d.stateMu.Lock()
	sessions := make([]SessionStatus, 0, len(d.sessions))
	for _, session := range d.sessions {
		sessions = append(sessions, session)
	}
	d.stateMu.Unlock()
	uptime := int64(0)
	if !d.started.IsZero() {
		uptime = time.Since(d.started).Milliseconds()
	}
	return Status{DaemonVersion: d.config.DaemonVersion, Protocol: ProtocolVersion, UptimeMS: uptime, EventsStored: d.stored.Load(), Sessions: sessions}
}

func decodeRequest(line []byte) (Request, error) {
	var request Request
	if err := json.Unmarshal(line, &request); err != nil {
		return Request{}, fmt.Errorf("decode request: %w", err)
	}
	if request.JSONRPC != "2.0" || request.Method == "" {
		return request, fmt.Errorf("invalid json-rpc request")
	}
	return request, nil
}

func writeResponse(writer io.Writer, response Response) error {
	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_, err = writer.Write(append(encoded, '\n'))
	return err
}

func rpcError(code int, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

func removeSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect daemon socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("daemon socket path is not a socket: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale daemon socket: %w", err)
	}
	return nil
}

func pidString(pid int) string { return strconv.Itoa(pid) }

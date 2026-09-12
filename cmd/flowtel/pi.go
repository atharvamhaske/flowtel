package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/atharvamhaske/flowtel/internal/adapter/pi"
	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/internal/daemon"
)

func runPi(args []string) error {
	defaults, err := daemon.DefaultConfig()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("pi run", flag.ContinueOnError)
	socketPath := flags.String("socket", defaults.SocketPath, "local Unix socket path")
	if len(args) == 0 || args[0] != "run" {
		return fmt.Errorf("usage: flowtel pi run [--socket PATH] [pi args...]")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("parse pi run flags: %w", err)
	}

	bin := os.Getenv("FLOWTEL_PI")
	if bin == "" {
		bin, err = exec.LookPath("pi")
		if err != nil {
			return fmt.Errorf("find pi binary: %w", err)
		}
	}
	sessionDir := os.Getenv("PI_CODING_AGENT_SESSION_DIR")
	if sessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		sessionDir = filepath.Join(home, ".pi", "agent", "sessions")
	}

	before := snapshotSessions(sessionDir)
	cmd := exec.Command(bin, flags.Args()...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := cmd.Run()
	after := snapshotSessions(sessionDir)

	sessionFile := newestNewFile(before, after)
	if sessionFile == "" {
		if runErr != nil {
			return fmt.Errorf("run pi: %w", runErr)
		}
		return fmt.Errorf("pi produced no new session file under %s", sessionDir)
	}

	cfg, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("load flowtel config: %w", err)
	}
	slog.Debug("dialing daemon socket", "socket", *socketPath, "session_file", sessionFile)
	if err := forwardPiSession(*socketPath, sessionFile, cfg); err != nil {
		return fmt.Errorf("forward pi session to daemon: %w", err)
	}
	slog.Info("forwarded pi session to daemon", "session_file", sessionFile)
	return runErr
}

func snapshotSessions(dir string) map[string]time.Time {
	snapshot := make(map[string]time.Time)
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		snapshot[path] = info.ModTime()
		return nil
	})
	return snapshot
}

func newestNewFile(before, after map[string]time.Time) string {
	var newest string
	var newestTime time.Time
	for path, modTime := range after {
		if seenTime, ok := before[path]; ok && !modTime.After(seenTime) {
			continue
		}
		if newest == "" || modTime.After(newestTime) {
			newest, newestTime = path, modTime
		}
	}
	return newest
}

func forwardPiSession(socketPath, sessionFile string, cfg config.Config) error {
	file, err := os.Open(sessionFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	result, err := pi.NewParser(cfg.Profile, true).Parse(context.Background(), file)
	if err != nil {
		return fmt.Errorf("parse pi session: %w", err)
	}
	if len(result.Events) == 0 {
		return fmt.Errorf("pi session had no events to forward")
	}
	slog.Info("forwarding events to daemon", "count", len(result.Events))

	connection, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon not running at %s", socketPath)
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	scanner := newDaemonScanner(connection)

	requestID := 1
	if err := sendRequest(connection, requestID, "initialize", map[string]any{
		"protocol_version": daemon.ProtocolVersion,
		"client":           daemon.Client{Source: "pi", PID: os.Getpid()},
	}); err != nil {
		return err
	}
	if _, err := readResponse(scanner); err != nil {
		return fmt.Errorf("initialize daemon connection: %w", err)
	}

	sessionID := result.Events[0].SessionID
	for _, event := range result.Events {
		requestID++
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode pi event: %w", err)
		}
		envelope := daemon.Envelope{Source: "pi", SessionID: event.SessionID, Event: event.Name, Payload: payload}
		if err := sendRequest(connection, requestID, "event.log", envelope); err != nil {
			return err
		}
		if _, err := readResponse(scanner); err != nil {
			return fmt.Errorf("send event %s: %w", event.Name, err)
		}
		if event.SessionID != "" {
			sessionID = event.SessionID
		}
	}

	requestID++
	if err := sendRequest(connection, requestID, "session.flush", map[string]any{"session_id": sessionID, "timeout_ms": 5000}); err != nil {
		return err
	}
	_, err = readResponse(scanner)
	return err
}

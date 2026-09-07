// Package agents contains small adapters used by integration tests.
package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/atharvamhaske/flowtel/internal/adapter/pi"
	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

type Pi struct {
	Profile model.Profile
}

func (p Pi) Run(ctx context.Context, connection net.Conn, input io.Reader) error {
	if connection == nil {
		return fmt.Errorf("run pi: connection is nil")
	}
	result, err := pi.NewParser(p.Profile, false).Parse(ctx, input)
	if err != nil {
		return fmt.Errorf("parse pi fixture: %w", err)
	}
	reader := bufio.NewReader(connection)
	if err := p.call(ctx, connection, reader, 1, "initialize", map[string]any{"protocol_version": daemon.ProtocolVersion, "client": map[string]any{"source": "pi", "plugin_version": "test", "pid": os.Getpid()}}); err != nil {
		return err
	}
	requestID := 2
	for _, event := range result.Events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode pi event: %w", err)
		}
		params := daemon.Envelope{Source: "pi", SourceVersion: "test", PluginVersion: "test", SessionID: event.SessionID, Event: event.Name, Payload: payload}
		if err := p.call(ctx, connection, reader, requestID, "event.log", params); err != nil {
			return err
		}
		requestID++
	}
	return p.call(ctx, connection, reader, requestID, "session.flush", map[string]any{"session_id": sessionID(result.Events), "timeout_ms": 1000})
}

func (p Pi) call(ctx context.Context, connection net.Conn, reader *bufio.Reader, id int, method string, params any) error {
	request := daemon.Request{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf("%d", id)), Method: method}
	request.Params, _ = json.Marshal(params)
	encoded, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	if _, err := connection.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", method, err)
	}
	responseLine, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read %s response: %w", method, err)
	}
	var response daemon.Response
	if err := json.Unmarshal(responseLine, &response); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if response.Error != nil {
		return fmt.Errorf("%s failed: %s", method, response.Error.Message)
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	default:
		return nil
	}
}

func sessionID(events []model.Event) string {
	for _, event := range events {
		if event.SessionID != "" {
			return event.SessionID
		}
	}
	return "unknown"
}

// Package daemon implements Flowtel's local harness-to-daemon protocol.
package daemon

import (
	"encoding/json"
	"fmt"
)

const ProtocolVersion = 1

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Client struct {
	Source        string `json:"source"`
	PluginVersion string `json:"plugin_version,omitempty"`
	PID           int    `json:"pid,omitempty"`
}

type Envelope struct {
	Source        string          `json:"source"`
	SourceVersion string          `json:"source_version,omitempty"`
	SessionID     string          `json:"session_id"`
	Event         string          `json:"event"`
	TimestampMS   int64           `json:"ts_ms"`
	ManagedRunID  string          `json:"managed_run_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	Route         json.RawMessage `json:"route,omitempty"`
}

func (e Envelope) Validate() error {
	if e.Source == "" || e.SessionID == "" || e.Event == "" {
		return fmt.Errorf("envelope requires source, session_id, and event")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("envelope payload is required")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("envelope payload is not valid json")
	}
	if len(e.Route) > 0 && !json.Valid(e.Route) {
		return fmt.Errorf("envelope route is not valid json")
	}
	return nil
}

type Status struct {
	DaemonVersion string          `json:"daemon_version"`
	Protocol      int             `json:"protocol_version"`
	UptimeMS      int64           `json:"uptime_ms"`
	Queued        int             `json:"queued"`
	EventsStored  int64           `json:"events_stored"`
	Sessions      []SessionStatus `json:"sessions"`
}

type SessionStatus struct {
	SessionID string `json:"session_id"`
	Source    string `json:"source"`
	Queued    int    `json:"queued"`
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode params: %w", err)
	}
	return nil
}

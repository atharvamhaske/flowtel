// Package model defines Flowtel's vendor-neutral event contract.
package model

import (
	"errors"
	"fmt"
	"time"
)

const SchemaVersion = "0.1.0"

var ErrInvalidEvent = errors.New("model: invalid event")

type Profile string

const (
	ProfileUnknown       Profile = ""
	ProfileOpenInference Profile = "openinference"
	ProfileGenAI         Profile = "gen_ai"
	ProfileBoth          Profile = "both"
)

func (p Profile) Valid() bool {
	return p == ProfileOpenInference || p == ProfileGenAI || p == ProfileBoth
}

type Kind string

const (
	KindUnknown    Kind = ""
	KindSession    Kind = "session"
	KindAgent      Kind = "agent"
	KindTurn       Kind = "turn"
	KindLLM        Kind = "llm"
	KindTool       Kind = "tool"
	KindPermission Kind = "permission"
	KindCompaction Kind = "compaction"
)

type Event struct {
	ID                 string
	ParentID           string
	SessionID          string
	Harness            string
	Profile            Profile
	Kind               Kind
	Name               string
	Start              time.Time
	End                time.Time
	Model              string
	Provider           string
	ToolName           string
	PermissionDecision string
	PermissionSource   string
	Error              string
	InputTokens        int64
	OutputTokens       int64
	ReasoningTokens    int64
	CacheReadTokens    int64
}

func (e Event) Validate() error {
	if e.ID == "" || e.Harness == "" || e.Profile == ProfileUnknown || !e.Profile.Valid() || e.Kind == KindUnknown || e.Name == "" {
		return ErrInvalidEvent
	}
	if !e.Start.IsZero() && !e.End.IsZero() && e.End.Before(e.Start) {
		return fmt.Errorf("%w: end before start", ErrInvalidEvent)
	}
	if e.Kind == KindPermission && e.PermissionDecision == "" {
		return fmt.Errorf("%w: permission decision is required", ErrInvalidEvent)
	}
	return nil
}

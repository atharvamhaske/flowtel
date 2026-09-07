package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

var ErrMalformed = errors.New("pi: malformed session input")

type Result struct {
	Events []model.Event
	Audit  []audit.Record
}

type Parser struct {
	Profile    model.Profile
	BestEffort bool
}

func NewParser(profile model.Profile, bestEffort bool) Parser {
	return Parser{Profile: profile, BestEffort: bestEffort}
}

func (p Parser) Parse(ctx context.Context, input io.Reader) (Result, error) {
	if input == nil {
		return Result{}, fmt.Errorf("parse pi session: %w", ErrMalformed)
	}
	if !p.Profile.Valid() {
		return Result{}, fmt.Errorf("parse pi session: invalid profile %q", p.Profile)
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	result := Result{}
	lineNumber := 0
	var sessionID string
	for scanner.Scan() {
		lineNumber++
		select {
		case <-ctx.Done():
			return Result{}, fmt.Errorf("parse pi session: %w", ctx.Err())
		default:
		}

		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			if !p.BestEffort {
				return Result{}, p.failure(lineNumber, err)
			}
			result.Audit = append(result.Audit, p.failureRecord(lineNumber))
			continue
		}
		if sessionID == "" {
			sessionID = stringValue(entry, "sessionId")
			if sessionID == "" && stringValue(entry, "type") == "session" {
				sessionID = stringValue(entry, "id")
			}
		}
		events := p.eventsFromEntry(entry, sessionID, lineNumber)
		result.Events = append(result.Events, events...)
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("read pi session: %w", err)
	}
	return result, nil
}

func (p Parser) eventsFromEntry(entry map[string]any, sessionID string, lineNumber int) []model.Event {
	id := stringValue(entry, "id")
	if id == "" {
		id = "line-" + strconv.Itoa(lineNumber)
	}
	parentID := stringValue(entry, "parentId")
	kind := model.KindUnknown
	name := ""
	typ := stringValue(entry, "type")
	switch typ {
	case "session":
		kind, name = model.KindSession, "flowtel.session"
	case "permission":
		kind, name = model.KindPermission, "flowtel.permission"
	case "flowtel":
		kind = model.Kind(stringValue(entry, "kind"))
		name = stringValue(entry, "name")
	case "message":
		message, _ := entry["message"].(map[string]any)
		role := stringValue(message, "role")
		if role == "assistant" && hasUsage(message) {
			kind, name = model.KindLLM, "llm"
		} else if role == "toolResult" || hasToolCall(message) {
			kind, name = model.KindTool, "tool.unknown"
		}
	}
	if kind == model.KindUnknown {
		return nil
	}
	event := model.Event{
		ID: id, ParentID: parentID, SessionID: sessionID, Harness: "pi",
		Profile: p.Profile, Kind: kind, Name: name,
		Start: timestamp(entry), End: timestamp(entry),
		Model:              stringValue(entry, "model"),
		Provider:           stringValue(entry, "provider"),
		ToolName:           stringValue(entry, "toolName"),
		PermissionDecision: stringValue(entry, "decision"),
		PermissionSource:   stringValue(entry, "source"),
	}
	if event.Kind == model.KindTool && event.Name == "" {
		event.Name = "tool.unknown"
	}
	if kind == model.KindPermission && event.PermissionDecision == "" {
		event.PermissionDecision = stringValue(entry, "status")
	}
	return []model.Event{event}
}

func (p Parser) failure(line int, err error) error {
	return fmt.Errorf("parse pi session line %d: %w: %v", line, ErrMalformed, err)
}

func (p Parser) failureRecord(line int) audit.Record {
	return audit.Record{
		Name: "flowtel.parser.failed",
		Attributes: map[string]any{
			"flowtel.schema.version":    model.SchemaVersion,
			"flowtel.event.name":        "flowtel.parser.failed",
			"flowtel.harness":           "pi",
			"flowtel.attribute.profile": string(p.Profile),
			"flowtel.parser.line":       int64(line),
		},
	}
}

func stringValue(values map[string]any, key string) string {
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func hasUsage(message map[string]any) bool {
	usage, ok := message["usage"].(map[string]any)
	return ok && len(usage) > 0
}

func hasToolCall(message map[string]any) bool {
	content, ok := message["content"].([]any)
	if !ok {
		return false
	}
	for _, item := range content {
		block, ok := item.(map[string]any)
		if ok && stringValue(block, "type") == "toolCall" {
			return true
		}
	}
	return false
}

func timestamp(values map[string]any) time.Time {
	value := values["timestamp"]
	switch typed := value.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		if err == nil {
			return parsed
		}
	case float64:
		return time.Unix(0, int64(typed)*int64(time.Millisecond))
	}
	return time.Time{}
}

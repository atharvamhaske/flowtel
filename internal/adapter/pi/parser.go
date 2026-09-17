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

type parseState struct {
	sessionID  string
	agentID    string
	turnID     string
	lastLLM    string
	turnSeq    int
	compactSeq int
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
	state := &parseState{}
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
		state.rememberSession(entry)
		events := p.eventsFromEntry(entry, state, lineNumber)
		result.Events = append(result.Events, events...)
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("read pi session: %w", err)
	}
	return result, nil
}

func (p Parser) eventsFromEntry(entry map[string]any, state *parseState, lineNumber int) []model.Event {
	id := stringValue(entry, "id")
	if id == "" {
		id = "line-" + strconv.Itoa(lineNumber)
	}
	parentID := firstString(stringValue(entry, "parentId"), stringValue(entry, "parent_id"))
	kind := model.KindUnknown
	name := ""
	typ := firstString(stringValue(entry, "type"), stringValue(entry, "event"))
	native := entry
	if payload, ok := entry["payload"].(map[string]any); ok {
		native = payload
	}
	switch typ {
	case "session", "session_start", "session_shutdown":
		kind, name = model.KindSession, "flowtel.session"
	case "before_agent_start", "agent_start":
		kind, name = model.KindAgent, "flowtel.agent"
		if state.agentID == "" {
			state.agentID = firstString(id, "agent:"+state.sessionID)
		}
		id = state.agentID
		if parentID == "" {
			parentID = state.sessionID
		}
	case "agent_end", "agent_settled":
		kind, name = model.KindAgent, "flowtel.agent"
		id = firstString(state.agentID, id)
		if parentID == "" {
			parentID = state.sessionID
		}
	case "turn_start":
		state.turnSeq++
		state.turnID = firstString(id, "turn:"+state.sessionID+":"+strconv.Itoa(state.turnSeq))
		kind, name = model.KindTurn, "flowtel.turn"
		id = state.turnID
		if parentID == "" {
			parentID = firstString(state.agentID, state.sessionID)
		}
	case "turn_end":
		kind, name = model.KindTurn, "flowtel.turn"
		id = firstString(state.turnID, id)
		if parentID == "" {
			parentID = firstString(state.agentID, state.sessionID)
		}
	case "permission":
		kind, name = model.KindPermission, "flowtel.permission"
	case "flowtel":
		kind = model.Kind(stringValue(entry, "kind"))
		name = stringValue(entry, "name")
	case "message", "message_end":
		message, _ := firstMap(entry["message"], native["message"])
		role := stringValue(message, "role")
		if role == "assistant" && (hasUsage(message) || hasUsage(native) || typ == "message_end") {
			kind, name = model.KindLLM, "llm"
		} else if role == "toolResult" || hasToolCall(message) {
			kind, name = model.KindTool, "tool.unknown"
		}
	case "message_start":
		message, _ := firstMap(entry["message"], native["message"])
		if stringValue(message, "role") == "assistant" {
			kind, name = model.KindLLM, "llm"
		}
	case "tool_execution_start", "tool_execution_end":
		kind, name = model.KindTool, "tool.unknown"
	case "compaction_start", "compaction_end":
		kind, name = model.KindCompaction, "flowtel.compaction"
		if typ == "compaction_start" {
			state.compactSeq++
		}
		id = firstString(id, "compaction:"+state.sessionID+":"+strconv.Itoa(state.compactSeq))
		if parentID == "" {
			parentID = firstString(state.turnID, state.agentID, state.sessionID)
		}
	}
	if kind == model.KindUnknown {
		return nil
	}
	if parentID == "" {
		parentID = state.defaultParent(kind)
	}
	message, _ := firstMap(entry["message"], native["message"])
	event := model.Event{
		ID: id, ParentID: parentID, SessionID: state.sessionID, Harness: "pi",
		Profile: p.Profile, Kind: kind, Name: name,
		Start: timestamp(entry), End: timestamp(entry),
		Model:              stringValue(entry, "model"),
		Provider:           stringValue(entry, "provider"),
		ToolName:           stringValue(entry, "toolName"),
		PermissionDecision: stringValue(entry, "decision"),
		PermissionSource:   stringValue(entry, "source"),
		ThinkingChars:      thinkingChars(message),
	}
	if event.SessionID == "" {
		event.SessionID = stringValue(native, "session_id")
	}
	if event.Kind == model.KindLLM {
		message, _ := firstMap(native["message"], entry["message"])
		event.Model = firstString(event.Model, stringValue(message, "model"))
		event.Provider = firstString(event.Provider, stringValue(message, "provider"))
		event.InputTokens, event.OutputTokens, event.ReasoningTokens, event.CacheReadTokens, event.CacheWriteTokens = usageTokens(entry, native, message)
		event.CostInput, event.CostOutput, event.CostCacheRead, event.CostCacheWrite, event.CostTotal = usageCost(entry, native, message)
		if stringValue(message, "stopReason") == "error" {
			event.Error = firstString(stringValue(message, "errorMessage"), "llm call failed")
		}
		state.lastLLM = event.ID
		if event.ParentID == "" {
			event.ParentID = firstString(state.turnID, state.agentID, state.sessionID)
		}
	}
	if event.Kind == model.KindTool {
		event.ID = firstString(stringValue(entry, "toolCallId"), stringValue(entry, "tool_call_id"), event.ID)
		event.ToolName = firstString(stringValue(entry, "toolName"), stringValue(entry, "tool_name"), event.ToolName)
		if event.ToolName != "" {
			event.Name = "tool." + event.ToolName
		}
		if event.Name == "" {
			event.Name = "tool.unknown"
		}
		if stderr := toolError(entry, native); stderr != "" {
			event.Error = stderr
		} else if boolValue(entry, "isError") {
			event.Error = "tool execution failed"
		}
		if event.ParentID == "" {
			event.ParentID = firstString(state.turnID, state.lastLLM, state.agentID, state.sessionID)
		}
	}
	if event.Kind == model.KindPermission && event.PermissionDecision == "" {
		event.PermissionDecision = stringValue(entry, "status")
	}
	return []model.Event{event}
}

func (s *parseState) rememberSession(entry map[string]any) {
	if s.sessionID != "" {
		return
	}
	s.sessionID = firstString(stringValue(entry, "sessionId"), stringValue(entry, "session_id"))
	if s.sessionID == "" && (stringValue(entry, "type") == "session" || stringValue(entry, "event") == "session_start") {
		s.sessionID = stringValue(entry, "id")
	}
}

func (s *parseState) defaultParent(kind model.Kind) string {
	switch kind {
	case model.KindAgent:
		return s.sessionID
	case model.KindTurn:
		return firstString(s.agentID, s.sessionID)
	case model.KindLLM, model.KindCompaction:
		return firstString(s.turnID, s.agentID, s.sessionID)
	case model.KindTool, model.KindPermission:
		return firstString(s.turnID, s.lastLLM, s.agentID, s.sessionID)
	default:
		return s.sessionID
	}
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

// thinkingChars sums the character length of every "thinking" content block
// on the message, so a step's reasoning presence/size is visible without
// exporting the reasoning text itself — the text is a raw model payload,
// and SPEC.md defaults to no raw payloads.
func thinkingChars(message map[string]any) int64 {
	content, ok := message["content"].([]any)
	if !ok {
		return 0
	}
	var total int64
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok || stringValue(block, "type") != "thinking" {
			continue
		}
		total += int64(len(stringValue(block, "thinking")))
	}
	return total
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstMap(values ...any) (map[string]any, bool) {
	for _, value := range values {
		if typed, ok := value.(map[string]any); ok {
			return typed, true
		}
	}
	return nil, false
}

func numberValue(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return int64(value)
		case int64:
			return value
		}
	}
	return 0
}

func floatValue(values map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return value
		case int64:
			return float64(value)
		}
	}
	return 0
}

func boolValue(values map[string]any, key string) bool {
	value, ok := values[key].(bool)
	return ok && value
}

func usageTokens(entry, native, message map[string]any) (input, output, reasoning, cacheRead, cacheWrite int64) {
	usage, _ := firstMap(native["usage"], message["usage"], entry["usage"])
	if usage == nil {
		return 0, 0, 0, 0, 0
	}
	return numberValue(usage, "input", "input_tokens", "prompt_tokens"),
		numberValue(usage, "output", "output_tokens", "completion_tokens"),
		numberValue(usage, "reasoning", "reasoning_tokens"),
		numberValue(usage, "cacheRead", "cache_read", "cacheReadTokens"),
		numberValue(usage, "cacheWrite", "cache_write", "cacheWriteTokens")
}

// usageCost reads pi's usage.cost block (dollar amounts, one field per
// token category). Not every harness/model reports cost, so a missing
// block just means all-zero, same as missing token counts.
func usageCost(entry, native, message map[string]any) (input, output, cacheRead, cacheWrite, total float64) {
	usage, _ := firstMap(native["usage"], message["usage"], entry["usage"])
	if usage == nil {
		return 0, 0, 0, 0, 0
	}
	cost, _ := usage["cost"].(map[string]any)
	if cost == nil {
		return 0, 0, 0, 0, 0
	}
	return floatValue(cost, "input"), floatValue(cost, "output"),
		floatValue(cost, "cacheRead", "cache_read"), floatValue(cost, "cacheWrite", "cache_write"),
		floatValue(cost, "total")
}

func toolError(entry, native map[string]any) string {
	result, _ := firstMap(entry["result"], native["result"])
	return firstString(stringValue(entry, "stderr"), stringValue(native, "stderr"), stringValue(result, "stderr"))
}

func timestamp(values map[string]any) time.Time {
	if milliseconds, ok := values["ts_ms"].(float64); ok {
		return time.Unix(0, int64(milliseconds)*int64(time.Millisecond))
	}
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

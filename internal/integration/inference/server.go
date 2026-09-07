// Package inference provides deterministic model-protocol test servers.
package inference

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/atharvamhaske/flowtel/internal/integration/server"
)

type Scenario struct {
	OpenAIResponse    any
	AnthropicResponse any
}

type Request struct {
	Path string
	Body []byte
}

type Server struct {
	host     *server.Server
	scenario Scenario
	mu       sync.Mutex
	requests []Request
}

func New(scenario Scenario) *Server {
	mock := &Server{scenario: scenario}
	mock.host = server.New(http.HandlerFunc(mock.handle))
	return mock
}

func (s *Server) URL() string { return s.host.URL() }

func (s *Server) Close() { s.host.Close() }

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := make([]Request, len(s.requests))
	copy(requests, s.requests)
	return requests
}

func (s *Server) handle(response http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.requests = append(s.requests, Request{Path: request.URL.Path, Body: append([]byte(nil), body...)})
	s.mu.Unlock()
	if request.URL.Path == "/v1/chat/completions" {
		writeChatCompletion(response)
		return
	}
	var value any
	switch request.URL.Path {
	case "/v1/responses":
		value = s.scenario.OpenAIResponse
	case "/v1/messages":
		value = s.scenario.AnthropicResponse
	default:
		http.NotFound(response, request)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	if value == nil {
		value = map[string]any{"id": "mock-response", "output": []any{}}
	}
	if err := json.NewEncoder(response).Encode(value); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	}
}

func writeChatCompletion(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := response.(http.Flusher)
	chunks := []map[string]any{
		{"id": "chatcmpl-mock", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "hi"}}}},
		{"id": "chatcmpl-mock", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 2, "completion_tokens": 1, "total_tokens": 3}},
	}
	for _, chunk := range chunks {
		encoded, err := json.Marshal(chunk)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := response.Write([]byte("data: " + string(encoded) + "\n\n")); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	_, _ = response.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

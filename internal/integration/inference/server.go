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

// Package ingest captures OTLP-like requests for integration assertions.
package ingest

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/atharvamhaske/flowtel/internal/integration/server"
)

type Row struct {
	Path string
	Body []byte
}

type Scenario struct {
	Paths []string
}

type Server struct {
	host   *server.Server
	mu     sync.Mutex
	rows   []Row
	status int
	stall  time.Duration
}

func New() *Server {
	mock := &Server{}
	mock.host = server.New(http.HandlerFunc(mock.handle))
	return mock
}

func (s *Server) URL() string { return s.host.URL() }

func (s *Server) Close() { s.host.Close() }

func (s *Server) Rows() []Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]Row, len(s.rows))
	copy(rows, s.rows)
	return rows
}

func (s *Server) Fail(status int) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

func (s *Server) Stall(delay time.Duration) {
	s.mu.Lock()
	s.stall = delay
	s.mu.Unlock()
}

func (s *Server) Matches(scenario Scenario) bool {
	rows := s.Rows()
	position := 0
	for _, expected := range scenario.Paths {
		for position < len(rows) && rows[position].Path != expected {
			position++
		}
		if position == len(rows) {
			return false
		}
		position++
	}
	return true
}

func (s *Server) handle(response http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	stall := s.stall
	status := s.status
	s.mu.Unlock()
	if stall > 0 {
		time.Sleep(stall)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.rows = append(s.rows, Row{Path: request.URL.Path, Body: append([]byte(nil), body...)})
	s.mu.Unlock()
	if status >= http.StatusBadRequest {
		response.WriteHeader(status)
		return
	}
	response.WriteHeader(http.StatusAccepted)
}

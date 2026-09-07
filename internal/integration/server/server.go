// Package server owns the lifecycle of a test HTTP server.
package server

import "net/http"
import "net/http/httptest"

type Server struct {
	server *httptest.Server
}

func New(handler http.Handler) *Server {
	return &Server{server: httptest.NewServer(handler)}
}

func (s *Server) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func (s *Server) Close() {
	if s != nil && s.server != nil {
		s.server.Close()
	}
}

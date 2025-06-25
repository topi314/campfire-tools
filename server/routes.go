package server

import (
	"log/slog"
	"net/http"
)

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	if err := s.templates().ExecuteTemplate(w, "index.gohtml", nil); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render index template", slog.String("error", err.Error()))
	}
}

func (s *Server) NotFound(w http.ResponseWriter, r *http.Request) {
	if err := s.templates().ExecuteTemplate(w, "not_found.gohtml", nil); err != nil {
		slog.ErrorContext(r.Context(), "Failed to render not found template", slog.String("error", err.Error()))
		return
	}
}

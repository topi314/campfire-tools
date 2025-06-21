package server

import (
	"net/http"
)

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, "")
}

func (s *Server) renderIndex(w http.ResponseWriter, errorMessage string) {
	if err := s.templates.ExecuteTemplate(w, "index.gohtml", map[string]any{
		"Error": errorMessage,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

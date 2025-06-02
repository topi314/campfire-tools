package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

var (
	//go:embed static
	static embed.FS

	//go:embed templates/*.gohtml
	templates embed.FS
)

func New(address string) *Server {
	t := template.Must(template.New("templates").
		ParseFS(templates, "templates/*.gohtml"),
	)

	mux := http.NewServeMux()

	s := &Server{
		Server: &http.Server{
			Addr:    address,
			Handler: mux,
		},
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		Templates: t,
	}

	mux.HandleFunc("/", s.Index)
	mux.HandleFunc("GET /raffle", s.Raffle)
	mux.Handle("/static/", http.FileServer(http.FS(static)))
	//mux.Handle("/*", http.RedirectHandler("/", http.StatusFound))

	return s
}

type Server struct {
	Server    *http.Server
	Client    *http.Client
	Templates *template.Template
}

func (s *Server) Start() {
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server failed: %s\n", err)
		}
	}()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown failed: %s\n", err)
		return
	}
	log.Println("Server stopped gracefully")
}

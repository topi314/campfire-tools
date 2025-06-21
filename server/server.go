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

	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

var (
	//go:embed static
	static embed.FS

	//go:embed templates/*.gohtml
	templates embed.FS
)

func New(cfg Config) (*Server, error) {
	t := template.Must(template.New("templates").
		ParseFS(templates, "templates/*.gohtml"),
	)

	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	mux := http.NewServeMux()

	s := &Server{
		server: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: mux,
		},
		client:    campfire.New(),
		database:  db,
		templates: t,
	}

	mux.HandleFunc("/", s.Index)

	mux.HandleFunc("GET /raffle", s.Raffle)
	mux.HandleFunc("GET /raffle/result", s.RaffleResult)

	mux.HandleFunc("GET /export", s.Export)
	mux.HandleFunc("GET /export/csv", s.ExportCSV)

	mux.HandleFunc("GET /tracker", s.Tracker)
	mux.HandleFunc("GET /tracker/club/{club_id}", s.TrackerClub)
	mux.HandleFunc("GET /tracker/add", s.TrackerAdd)

	mux.Handle("/static/", http.FileServer(http.FS(static)))

	return s, nil
}

type Server struct {
	server    *http.Server
	client    *campfire.Client
	database  *database.Database
	templates *template.Template
}

func (s *Server) Start() {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server failed: %s\n", err)
		}
	}()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown failed: %s\n", err)
		return
	}
}

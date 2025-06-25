package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path"
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
	var staticFS http.FileSystem
	var t func() *template.Template
	if cfg.Dev {
		root, err := os.OpenRoot("server/")
		if err != nil {
			return nil, fmt.Errorf("failed to open static directory: %w", err)
		}
		staticFS = http.FS(root.FS())
		t = func() *template.Template {
			return template.Must(template.New("templates").
				Funcs(templateFuncs).
				ParseFS(root.FS(), "templates/*.gohtml"))
		}
	} else {
		staticFS = http.FS(static)

		st := template.Must(template.New("templates").
			Funcs(templateFuncs).
			ParseFS(templates, "templates/*.gohtml"),
		)

		t = func() *template.Template {
			return st
		}
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	mux := http.NewServeMux()
	httpClient := &http.Client{}

	s := &Server{
		server: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: cleanPathMiddleware(mux),
		},
		httpClient: httpClient,
		client:     campfire.New(cfg.Campfire, httpClient),
		database:   db,
		templates:  t,
	}

	fs := http.FileServer(staticFS)

	mux.HandleFunc("/", s.NotFound)
	mux.HandleFunc("GET /{$}", s.Index)
	mux.HandleFunc("GET /raffle", s.Raffle)
	mux.HandleFunc("POST /raffle", s.DoRaffle)
	mux.HandleFunc("GET /export", s.Export)
	mux.HandleFunc("POST /export", s.DoExport)
	mux.HandleFunc("GET /tracker", s.Tracker)
	mux.HandleFunc("POST /tracker", s.TrackerAdd)
	mux.HandleFunc("GET /tracker/club/{club_id}", s.TrackerClub)
	mux.HandleFunc("GET /tracker/club/{club_id}/export", s.TrackerClubExport)
	mux.HandleFunc("POST /tracker/club/{club_id}/export", s.DoTrackerClubExport)
	mux.HandleFunc("GET /tracker/club/{club_id}/member/{member_id}", s.TrackerClubMember)
	mux.HandleFunc("GET /tracker/event/{event_id}", s.TrackerClubEvent)

	mux.HandleFunc("GET /images/{image_id}", s.Image)
	mux.Handle("GET /static/", fs)
	mux.Handle("HEAD /static/", fs)

	return s, nil
}

func cleanPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the request URL path
		r.URL.Path = path.Clean(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

type Server struct {
	server     *http.Server
	httpClient *http.Client
	client     *campfire.Client
	database   *database.Database
	templates  func() *template.Template
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
		slog.Error("Server shutdown failed", slog.Any("err", err))
		return
	}
}

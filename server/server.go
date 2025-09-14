package server

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/database"
)

var (
	//go:embed web/static
	static embed.FS

	//go:embed web/templates/*.gohtml
	templates embed.FS
)

func New(cfg Config) (*Server, error) {
	var staticFS http.FileSystem
	var t func() *template.Template
	if cfg.Dev {
		root, err := os.OpenRoot("server/web/")
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
		subStaticFS, err := fs.Sub(static, "web")
		if err != nil {
			return nil, fmt.Errorf("failed to create sub FS for static files: %w", err)
		}
		staticFS = http.FS(subStaticFS)

		st := template.Must(template.New("templates").
			Funcs(templateFuncs).
			ParseFS(templates, "web/templates/*.gohtml"),
		)

		t = func() *template.Template {
			return st
		}
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	httpClient := &http.Client{}
	s := &Server{
		Cfg: cfg,
		Server: &http.Server{
			Addr: cfg.Server.Addr,
		},
		HttpClient: httpClient,
		Campfire:   campfire.New(cfg.Campfire, httpClient, getCampfireToken(db)),
		DB:         db,
		Auth:       auth.New(cfg.Auth, db),
		Templates:  t,
		StaticFS:   staticFS,
	}

	return s, nil
}

func getCampfireToken(db *database.Database) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		token, err := db.GetNextCampfireToken(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", errors.New("oooops, no valid token found. Please ping me on Discord with this error")
			}
			return "", fmt.Errorf("failed to get next campfire token: %w", err)
		}

		return token.Token, nil
	}
}

func cleanPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the request URL path
		r.URL.Path = path.Clean(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

type Server struct {
	Cfg        Config
	Server     *http.Server
	HttpClient *http.Client
	Campfire   *campfire.Client
	DB         *database.Database
	Auth       *auth.Auth
	Templates  func() *template.Template
	StaticFS   http.FileSystem
}

func (s *Server) Start(handler http.Handler) {
	s.Server.Handler = cleanPathMiddleware(handler)
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server failed: %s\n", err)
		}
	}()

	go s.importClubs()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", slog.Any("err", err))
		return
	}
}

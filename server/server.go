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

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"

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

	tmplFuncs := make(template.FuncMap, len(templateFuncs)+1)
	for name, fn := range templateFuncs {
		tmplFuncs[name] = fn
	}
	tmplFuncs["devEnabled"] = func() bool {
		return cfg.Dev
	}

	if cfg.Dev {
		root, err := os.OpenRoot("server/web/")
		if err != nil {
			return nil, fmt.Errorf("failed to open static directory: %w", err)
		}
		staticFS = http.FS(root.FS())
		t = func() *template.Template {
			return template.Must(template.New("templates").
				Funcs(tmplFuncs).
				ParseFS(root.FS(), "templates/*.gohtml"))
		}
	} else {
		subStaticFS, err := fs.Sub(static, "web")
		if err != nil {
			return nil, fmt.Errorf("failed to create sub FS for static files: %w", err)
		}
		staticFS = http.FS(subStaticFS)

		st := template.Must(template.New("templates").
			Funcs(tmplFuncs).
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

	var webhookClient *webhook.Client
	if cfg.Notifications.Enabled {
		webhookClient, err = webhook.NewWithURL(cfg.Notifications.WebhookURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create webhook client: %w", err)
		}

		wh, err := webhookClient.GetWebhook()
		if err != nil {
			return nil, fmt.Errorf("failed to verify webhook URL: %w", err)
		}
		slog.Info("Discord webhook notifications enabled", slog.String("name", wh.Name()), slog.String("guild_id", wh.GuildID.String()), slog.String("channel_id", wh.ChannelID.String()))
	}

	httpClient := &http.Client{}
	var (
		reloadNotifier *reloadNotifier
		cancelWatcher  context.CancelFunc
	)

	if cfg.Dev {
		reloadNotifier = newReloadNotifier()
		cancelWatcher = startDevWatcher("server/web", reloadNotifier)
	}

	s := &Server{
		Cfg: cfg,
		Server: &http.Server{
			Addr: cfg.Server.Addr,
		},
		HttpClient:    httpClient,
		Campfire:      campfire.New(cfg.Campfire, httpClient, getCampfireToken(db)),
		DB:            db,
		Auth:          auth.New(cfg.Auth, db),
		Templates:     t,
		StaticFS:      staticFS,
		WebhookClient: webhookClient,
		ReloadNotifier: reloadNotifier,
		devWatcherCancel: cancelWatcher,
	}

	go s.cleanup()

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
	Cfg                    Config
	Server                 *http.Server
	HttpClient             *http.Client
	Campfire               *campfire.Client
	DB                     *database.Database
	Auth                   *auth.Auth
	Templates              func() *template.Template
	StaticFS               http.FileSystem
	WebhookClient          *webhook.Client
	SentTokenNotifications []int
	ReloadNotifier         *reloadNotifier
	devWatcherCancel       context.CancelFunc
}

func (s *Server) Start(handler http.Handler) {
	s.Server.Handler = cleanPathMiddleware(handler)
	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server failed: %s\n", err)
		}
	}()

	go s.importClubs()
	go s.importEvents()
	go s.updateEvents()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", slog.Any("err", err))
		return
	}

	if s.devWatcherCancel != nil {
		s.devWatcherCancel()
	}

	if s.ReloadNotifier != nil {
		s.ReloadNotifier.Close()
	}
}

func (s *Server) SendNotification(ctx context.Context, content string) {
	if s.WebhookClient == nil {
		slog.WarnContext(ctx, content)
		return
	}

	if _, err := s.WebhookClient.CreateMessage(discord.WebhookMessageCreate{
		Flags: discord.MessageFlagIsComponentsV2,
		Components: []discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay(content),
			).WithAccentColor(0xfe812e),
		},
	}, rest.CreateWebhookMessageParams{
		WithComponents: true,
		Wait:           false,
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to send notification", slog.Any("err", err), slog.String("content", content))
	}
}

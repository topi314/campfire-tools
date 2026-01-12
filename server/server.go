package server

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
	"github.com/topi314/goreload"

	"github.com/topi314/campfire-tools/server/auth"
	"github.com/topi314/campfire-tools/server/campfire"
	"github.com/topi314/campfire-tools/server/cauth"
	"github.com/topi314/campfire-tools/server/database"
)

const ReloadRoute = "/dev/reload"

var (
	//go:embed web/static
	static embed.FS

	//go:embed web
	templates embed.FS

	//go:embed web/static/campfire-tools-mini.png
	logo []byte
)

func New(cfg Config) (*Server, error) {
	reloader := goreload.New(goreload.Config{
		Logger:  slog.Default(),
		Route:   ReloadRoute,
		Enabled: cfg.Dev,
		MaxAge:  time.Hour,
	})

	var staticFS http.FileSystem
	var t func() *template.Template

	if cfg.Dev {
		root, err := os.OpenRoot("server/web/")
		if err != nil {
			return nil, fmt.Errorf("failed to open static directory: %w", err)
		}
		staticFS = http.FS(root.FS())
		t = func() *template.Template {
			return reloader.MustParseTemplate(template.Must(template.New("templates").
				Funcs(templateFuncs).
				ParseFS(root.FS(), "templates/*.gohtml", "tracker/templates/*.gohtml", "rewards/templates/*.gohtml", "homepage/templates/*.gohtml")))
		}
		reloader.Start(root.FS())
	} else {
		subStaticFS, err := fs.Sub(static, "web")
		if err != nil {
			return nil, fmt.Errorf("failed to create sub FS for static files: %w", err)
		}
		staticFS = http.FS(subStaticFS)

		st := reloader.MustParseTemplate(template.Must(template.New("templates").
			Funcs(templateFuncs).
			ParseFS(templates, "web/templates/*.gohtml", "web/tracker/templates/*.gohtml", "web/rewards/templates/*.gohtml", "web/homepage/templates/*.gohtml"),
		))

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

	logoPNG, err := png.Decode(bytes.NewReader(logo))
	if err != nil {
		return nil, fmt.Errorf("failed to decode logo: %w", err)
	}

	httpClient := &http.Client{}
	s := &Server{
		Cfg: cfg,
		TrackerServer: &http.Server{
			Addr: cfg.Server.TrackerAddr,
		},
		RewardsServer: &http.Server{
			Addr: cfg.Server.RewardsAddr,
		},
		HomepageServer: &http.Server{
			Addr: cfg.Server.HomepageAddr,
		},
		HttpClient:    httpClient,
		Campfire:      campfire.New(cfg.Campfire, httpClient, getCampfireToken(db)),
		DB:            db,
		Auth:          auth.New(cfg.DiscordAuth, cfg.Server.PublicTrackerURL),
		CampfireAuth:  cauth.New(cfg.CampfireAuth),
		Templates:     t,
		StaticFS:      staticFS,
		WebhookClient: webhookClient,
		Reloader:      reloader,
		Logo:          logoPNG,
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
	TrackerServer          *http.Server
	RewardsServer          *http.Server
	HomepageServer         *http.Server
	HttpClient             *http.Client
	Campfire               *campfire.Client
	DB                     *database.Database
	Auth                   *auth.Auth
	CampfireAuth           *cauth.Auth
	Templates              func() *template.Template
	StaticFS               http.FileSystem
	WebhookClient          *webhook.Client
	SentTokenNotifications []int
	Reloader               *goreload.Reloader
	Logo                   image.Image
}

func (s *Server) Start(trackerHandler http.Handler, rewardsHandler http.Handler, homepageHandler http.Handler) {
	s.TrackerServer.Handler = cleanPathMiddleware(trackerHandler)
	go func() {
		if err := s.TrackerServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Tracker server failed: %s\n", err)
		}
	}()

	s.RewardsServer.Handler = cleanPathMiddleware(rewardsHandler)
	go func() {
		if err := s.RewardsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Rewards server failed: %s\n", err)
		}
	}()

	s.HomepageServer.Handler = cleanPathMiddleware(homepageHandler)
	go func() {
		if err := s.HomepageServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Homepage server failed: %s\n", err)
		}
	}()

	go s.importClubs()
	go s.importEvents()
	go s.updateEvents()
}

func (s *Server) Stop() {
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	//
	// var wg sync.WaitGroup
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := s.TrackerServer.Shutdown(ctx); err != nil {
	// 		slog.Error("Tracker server shutdown failed", slog.Any("err", err))
	// 	}
	// }()
	//
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := s.RewardsServer.Shutdown(ctx); err != nil {
	// 		slog.Error("Rewards server shutdown failed", slog.Any("err", err))
	// 	}
	// }()
	//
	// wg.Wait()

	if s.Reloader != nil {
		s.Reloader.Close()
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

package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/topi314/campfire-tools/internal/xslog"
	"github.com/topi314/campfire-tools/server"
)

func main() {
	cfgPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	cfg, err := server.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("Error while loading config", slog.Any("err", err))
		return
	}

	setupLogger(cfg.Log)

	version := "unknown"
	goVersion := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		goVersion = info.GoVersion
	}

	slog.Info("Starting bpt...", slog.String("version", version), slog.String("go_version", goVersion))
	slog.Info("Config loaded", slog.Any("config", cfg.String()))

	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("Error while creating server", slog.Any("err", err))
		return
	}
	go srv.Start()
	defer srv.Stop()

	slog.Info("Server started", slog.String("addr", cfg.Server.Addr))

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM, syscall.SIGINT)
	<-s
}

func setupLogger(cfg server.LogConfig) {
	var handler slog.Handler
	switch cfg.Format {
	case server.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource:   cfg.AddSource,
			Level:       cfg.Level,
			ReplaceAttr: nil,
		})
	case server.LogFormatText:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource:   cfg.AddSource,
			Level:       cfg.Level,
			ReplaceAttr: nil,
		})
	default:
		slog.Error("Unknown log format", slog.String("format", string(cfg.Format)))
		os.Exit(-1)
	}

	slog.SetDefault(slog.New(xslog.NewFilterHandler(handler, filterContextCancelled)))
}

func filterContextCancelled(_ context.Context, record slog.Record) bool {
	shouldLog := true
	record.Attrs(func(attr slog.Attr) bool {
		if err, ok := attr.Value.Any().(error); ok {
			if errors.Is(err, context.Canceled) {
				shouldLog = false
			}
			return false
		}
		return true
	})
	return shouldLog
}

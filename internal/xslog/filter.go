package xslog

import (
	"context"
	"log/slog"
)

var _ slog.Handler = (*FilterHandler)(nil)

type FilterFunc func(ctx context.Context, record slog.Record) bool

func NewFilterHandler(handler slog.Handler, filter FilterFunc) *FilterHandler {
	return &FilterHandler{handler: handler, filter: filter}
}

type FilterHandler struct {
	handler slog.Handler
	filter  FilterFunc
}

func (f *FilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return f.handler.Enabled(ctx, level)
}

func (f *FilterHandler) Handle(ctx context.Context, record slog.Record) error {
	if f.filter != nil {
		if !f.filter(ctx, record) {
			return nil
		}
	}
	return f.handler.Handle(ctx, record)
}

func (f *FilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewFilterHandler(f.handler.WithAttrs(attrs), f.filter)
}

func (f *FilterHandler) WithGroup(name string) slog.Handler {
	return NewFilterHandler(f.handler.WithGroup(name), f.filter)
}

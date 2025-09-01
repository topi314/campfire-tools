package tsync

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

func ErrorGroupWithContext(ctx context.Context) (*ErrorGroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &ErrorGroup{cancel: cancel}, ctx
}

type ErrorGroup struct {
	sync.Mutex
	errors []error
	eg     errgroup.Group
	cancel context.CancelFunc
}

func (g *ErrorGroup) SetLimit(n int) {
	g.eg.SetLimit(n)
}

func (g *ErrorGroup) Go(n func() error) {
	g.eg.Go(func() error {
		if err := n(); err != nil {
			g.Lock()
			defer g.Unlock()
			g.errors = append(g.errors, err)
		}
		return nil
	})
}

func (g *ErrorGroup) Wait() error {
	_ = g.eg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return errors.Join(g.errors...)
}

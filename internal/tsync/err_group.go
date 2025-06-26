package tsync

import (
	"context"
	"sync"
)

func ErrorGroupWithContext(ctx context.Context) (*ErrorGroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &ErrorGroup{cancel: cancel}, ctx
}

type ErrorGroup struct {
	sync.Mutex
	errors []error
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func (g *ErrorGroup) Go(n func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := n(); err != nil {
			g.Lock()
			defer g.Unlock()
			g.errors = append(g.errors, err)
		}
	}()
}

func (g *ErrorGroup) Wait() []error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.errors
}

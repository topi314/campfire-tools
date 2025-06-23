package tsync

import (
	"sync"

	"golang.org/x/sync/errgroup"
)

type ErrorGroup struct {
	sync.Mutex
	errors []error
	eg     errgroup.Group
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

func (g *ErrorGroup) SetLimit(n int) {
	g.eg.SetLimit(n)
}

func (g *ErrorGroup) Wait() []error {
	_ = g.eg.Wait()
	return g.errors
}

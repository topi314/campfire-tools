package tsync

import (
	"sync"
)

type ErrorGroup struct {
	sync.Mutex
	errors []error
	wg     sync.WaitGroup
}

func (g *ErrorGroup) Go(n func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := n(); err != nil {
			g.Lock()
			g.errors = append(g.errors, err)
			g.Unlock()
		}
	}()
}

func (g *ErrorGroup) Wait() []error {
	g.wg.Wait()
	g.Lock()
	defer g.Unlock()
	return g.errors
}

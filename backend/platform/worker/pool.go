package worker

import (
	"context"
	"sync"
)

type Pool struct {
	jobs chan func(context.Context)
	wg   sync.WaitGroup
}

func New(ctx context.Context, size int) *Pool {
	p := &Pool{jobs: make(chan func(context.Context))}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go p.run(ctx)
	}
	return p
}

func (p *Pool) run(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			job(ctx)
		}
	}
}

func (p *Pool) Submit(ctx context.Context, job func(context.Context)) bool {
	select {
	case p.jobs <- job:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

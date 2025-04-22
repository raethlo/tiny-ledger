package main

import (
	"context"
	"log"
	"sync/atomic"
)

type Worker struct {
	Name    string
	Handler func(ctx context.Context) error
	OnPanic func(reason any)
	alive   atomic.Bool
	cancel  context.CancelFunc
}

func (w *Worker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.alive.Store(true)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[worker:%s] panic: %v", w.Name, r)
				w.alive.Store(false)
				if w.OnPanic != nil {
					w.OnPanic(r)
				}
			}
		}()

		log.Printf("[worker:%s] starting", w.Name)
		if err := w.Handler(ctx); err != nil {
			log.Printf("[worker:%s] error: %v", w.Name, err)
		}
		w.alive.Store(false)
	}()
}

func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *Worker) IsAlive() bool {
	return w.alive.Load()
}

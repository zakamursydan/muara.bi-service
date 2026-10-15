package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type App interface {
	BeforeStart(context.Context) error
	AfterStart(context.Context) error
	Shutdown(context.Context) error
}

type Container struct {
	App     App
	servers []*http.Server
	signal  <-chan os.Signal
	timeout time.Duration
}

type Option func(*Container)

func WithHTTPServer(servers []*http.Server) Option {
	return func(c *Container) {
		c.servers = append(c.servers, servers...)
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Container) {
		c.timeout = timeout
	}
}

var defaultSignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

func newSignalChannel() <-chan os.Signal {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, defaultSignals...)
	return sig
}

func New(app App, opts ...Option) *Container {
	c := &Container{
		App:     app,
		timeout: 30 * time.Second,
		signal:  newSignalChannel(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Container) Start(ctx context.Context) error {
	if err := c.App.BeforeStart(ctx); err != nil {
		return fmt.Errorf("app before start failed: %w", err)
	}

	errChan := make(chan error, len(c.servers))
	for _, srv := range c.servers {
		go func(s *http.Server) {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}(srv)
	}

	select {
	case sig := <-c.signal:
		fmt.Printf("[Container] Received termination signal: %v. Initiating graceful shutdown...\n", sig)
	case err := <-errChan:
		fmt.Printf("[Container] HTTP server error: %v. Initiating shutdown...\n", err)
	case <-ctx.Done():
		fmt.Println("[Container] Context canceled. Initiating shutdown...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	for _, srv := range c.servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("[Container] HTTP server shutdown error: %v\n", err)
		}
	}

	if err := c.App.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("[Container] App shutdown error: %v\n", err)
	}

	return c.App.AfterStart(ctx)
}

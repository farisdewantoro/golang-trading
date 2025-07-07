package cmd

import (
	"context"
	"fmt"
	"golang-trading/internal/delivery/http"
	"time"

	"go.uber.org/zap"
)

type HTTPServer struct {
	ctx     context.Context
	appDep  *AppDependency
	handler *http.HttpAPIHandler
}

func NewHTTPServer(ctx context.Context, appDep *AppDependency, handler *http.HttpAPIHandler) *HTTPServer {
	return &HTTPServer{
		ctx:     ctx,
		appDep:  appDep,
		handler: handler,
	}
}

func (s *HTTPServer) Start() error {
	s.appDep.log.Info("Starting HTTP server", zap.Int("port", s.appDep.cfg.API.Port))
	address := fmt.Sprintf(":%d", s.appDep.cfg.API.Port)

	s.SetupRoutes()

	return s.appDep.echo.Start(address)
}

func (s *HTTPServer) Stop() error {
	s.appDep.log.Info("Shutting down HTTP server")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// Stop the bot with timeout
	stopDone := make(chan error, 1)
	go func() {
		// Use a separate goroutine to avoid blocking
		err := s.appDep.echo.Shutdown(ctx)
		if err != nil {
			s.appDep.log.Error("Error When Stop HTTP server", zap.Error(err))
		}
		stopDone <- nil
	}()

	// Wait for server to stop with timeout
	select {
	case <-stopDone:
		s.appDep.log.Info("HTTP server stopped successfully")
	case <-ctx.Done():
		s.appDep.log.Warn("Timeout while stopping HTTP server, forcing shutdown")
	}
	return nil
}

func (s *HTTPServer) SetupRoutes() {
	s.handler.SetupRoutes()
}

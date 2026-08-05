package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
)

type Server struct {
	server                http.Server
	stopOngoingGracefully context.CancelFunc
	logger                *slog.Logger
}

const maxRequestBodySize = 1024 * 1024 // 1 MB

func NewServer(addr string, handler http.Handler, logger *slog.Logger) *Server {
	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())

	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "server")

	handler = http.MaxBytesHandler(handler, maxRequestBodySize)

	return &Server{
		server: http.Server{
			Addr:    addr,
			Handler: handler,
			BaseContext: func(_ net.Listener) context.Context {
				return ongoingCtx
			},
		},
		stopOngoingGracefully: stopOngoingGracefully,
		logger:                logger,
	}
}

func (s *Server) Start() {
	go func() {
		s.logger.Info("Starting accepting requests", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Fail while listening to new requests", "error", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down gracefully")
	err := s.server.Shutdown(ctx)
	s.stopOngoingGracefully()
	if err != nil {
		return err
	}
	return nil
}

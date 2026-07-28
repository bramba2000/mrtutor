package main

import (
	"context"
	"net"
	"net/http"
)

type Server struct {
	server                http.Server
	stopOngoingGracefully context.CancelFunc
}

func NewServer(addr string, handler http.Handler) *Server {
	ongoingCtx, stopOngoingGracefully := context.WithCancel(context.Background())

	return &Server{
		server: http.Server{
			Addr:    addr,
			Handler: handler,
			BaseContext: func(_ net.Listener) context.Context {
				return ongoingCtx
			},
		},
		stopOngoingGracefully: stopOngoingGracefully,
	}
}

func (s *Server) Start() {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	s.stopOngoingGracefully()
	if err != nil {
		return err
	}
	return nil
}

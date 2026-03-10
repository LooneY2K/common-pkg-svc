package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ServerConfig struct {
	IdleTimeout  int
	ReadTimeout  int
	WriteTimeout int
	ShutdownWait int
	Port         int
}

func StartAndGracefullShutdown(lgr *zap.SugaredLogger, router *chi.Mux, config ServerConfig) {
	s := &http.Server{
		Addr:         fmt.Sprintf("%s%d", ":", config.Port),
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		Handler:      router,
	}
	go func() {
		lgr.Info("starting server on port: ", config.Port)
		err := s.ListenAndServe()
		if err != nil {
			lgr.Fatal(err)
		}
	}()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// wait indefinitely until we don't receive an intrupt signal
	sig := <-signalChan
	lgr.Info("Received terminate, gracefully shutting down: ", sig)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.ShutdownWait)*time.Second)
	s.Shutdown(ctx)
	cancel()
}

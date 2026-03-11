package gin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LooneY2K/common-pkg-svc/server"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ServerConfig struct {
	IdleTimeout  int
	ReadTimeout  int
	WriteTimeout int
	ShutdownWait int
	Port         int
}

func StartAndGracefulShutdown(ctx context.Context, lgr *zap.SugaredLogger, router *gin.Engine, config server.ServerConfig) {

	s := &http.Server{
		Addr:         fmt.Sprintf("%s%d", ":", config.Port),
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		Handler:      router,
	}
	go func() {
		lgr.Info("starting server on port: ", zap.Int("port", config.Port))
		err := s.ListenAndServe()
		if err != nil {
			lgr.Error("failed to start server", zap.Error(err))
		}
	}()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// wait indefinitely until we receive an interrupt signal
	sig := <-signalChan
	lgr.Info("Received terminate, gracefully shutting down: ", zap.String("signal", sig.String()))
	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.ShutdownWait)*time.Second)
	err := s.Shutdown(ctx)
	if err != nil {
		lgr.Error("failed to shutdown server", zap.Error(err))
		panic("server shutdown failure")
	}
	cancel()
}

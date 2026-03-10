package gin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	elog "github.com/LooneY2K/common-pkg-svc/log/custom"
	"github.com/LooneY2K/common-pkg-svc/server"
	"github.com/gin-gonic/gin"
)

type ServerConfig struct {
	IdleTimeout  int
	ReadTimeout  int
	WriteTimeout int
	ShutdownWait int
	Port         int
}

func StartAndGracefulShutdown(ctx context.Context, lgr *elog.Logger, router *gin.Engine, config server.ServerConfig) {

	s := &http.Server{
		Addr:         fmt.Sprintf("%s%d", ":", config.Port),
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		Handler:      router,
	}
	go func() {
		lgr.Info("starting server on port: ", elog.Int("port", config.Port))
		err := s.ListenAndServe()
		if err != nil {
			lgr.Error("failed to start server", elog.Err(err))
		}
	}()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// wait indefinitely until we receive an interrupt signal
	sig := <-signalChan
	lgr.Info("Received terminate, gracefully shutting down: ", elog.String("signal", sig.String()))
	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.ShutdownWait)*time.Second)
	err := s.Shutdown(ctx)
	if err != nil {
		lgr.Error("failed to shutdown server", elog.Err(err))
		panic("server shutdown failure")
	}
	cancel()
}

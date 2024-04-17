package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-pckg/pine"
)

func main() {
	logger := pine.New()

	cfg, err := NewConfig()
	if err != nil {
		logger.Error("error initializing config", pine.Err(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { // catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		logger.Warn("Interrupt signal")
		cancel()
	}()

	app := serverApp{
		ServiceAURL: cfg.ServiceAURL,
		ServiceBURL: cfg.ServiceBURL,
		Port:        cfg.Port,
		Logger:      logger,
	}

	err = app.run(ctx)
	if err != nil {
		logger.Error("Proxy terminated with error", pine.Err(err))
	}

	logger.Info("Proxy terminated")
}

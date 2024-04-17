package main

import (
	"context"

	"github.com/glebteterin/gateway-proxy"
	"github.com/go-pckg/pine"
)

type serverApp struct {
	ServiceAURL string
	ServiceBURL string
	Port        string
	Logger      *pine.Logger

	server *gateway.Server
}

func (a *serverApp) run(ctx context.Context) error {
	go func() {
		// shutdown on context cancellation
		<-ctx.Done()
		a.Logger.Info("Shutdown initiated")
		a.server.Shutdown()
	}()

	server, err := gateway.NewServer(a.ServiceAURL, a.ServiceBURL, a.Logger)
	if err != nil {
		return err
	}

	a.Logger.Debugf("Service A: %s", a.ServiceAURL)
	a.Logger.Debugf("Service B: %s", a.ServiceBURL)
	a.Logger.Infof("Listening and serving http on :%s", a.Port)

	a.server = server
	server.Run(a.Port)
	return nil
}

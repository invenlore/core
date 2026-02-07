package metrics

import (
	"fmt"
	"net"
	"net/http"

	"github.com/invenlore/core/pkg/config"
	"github.com/sirupsen/logrus"
)

func StartMetricsServer(cfg *config.MetricsServerConfig, handler http.Handler) (*http.Server, net.Listener, error) {
	var (
		loggerEntry = logrus.WithField("scope", "metrics")
		listenAddr  = net.JoinHostPort(cfg.Host, cfg.Port)
	)

	loggerEntry.Info("starting metrics server...")

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	return server, ln, nil
}

package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/invenlore/proto/pkg/user"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type RegisterFunc func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

type GRPCService struct {
	Name     string
	Address  string
	Register RegisterFunc
}

type GRPCServerConfig struct {
	Host              string
	Port              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
}

type HTTPServerConfig struct {
	Host              string
	Port              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
}

type HealthServerConfig struct {
	Host string
	Port string
}

type AppConfig struct {
	AppEnv   string `env:"APP_ENV" envDefault:"dev"`
	LogLevel string `env:"APP_LOG_LEVEL" envDefault:"INFO"`

	GRPC struct {
		Host              string        `env:"GRPC_HOST" envDefault:"0.0.0.0"`
		Port              string        `env:"GRPC_PORT" envDefault:"8080"`
		ReadTimeout       time.Duration `env:"GRPC_READ_TIMEOUT" envDefault:"10s"`
		WriteTimeout      time.Duration `env:"GRPC_WRITE_TIMEOUT" envDefault:"10s"`
		IdleTimeout       time.Duration `env:"GRPC_IDLE_TIMEOUT" envDefault:"60s"`
		ReadHeaderTimeout time.Duration `env:"GRPC_READ_HEADER_TIMEOUT" envDefault:"5s"`
	} `envPrefix:"GRPC_"`

	HTTP struct {
		Host              string        `env:"HTTP_HOST" envDefault:"0.0.0.0"`
		Port              string        `env:"HTTP_PORT" envDefault:"8080"`
		ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s"`
		WriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"10s"`
		IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
		ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	} `envPrefix:"HTTP_"`

	Health struct {
		Host string `env:"HEALTH_HOST" envDefault:"0.0.0.0"`
		Port string `env:"HEALTH_PORT" envDefault:"80"`
	} `envPrefix:"HEALTH_"`

	GRPCServices []GRPCService `env:"-"`
}

type ConfigProvider interface {
	GetGRPCConfig() HTTPServerConfig
	GetHTTPConfig() HTTPServerConfig
	GetHealthConfig() HealthServerConfig
	GetGRPCServices() []GRPCService
}

type appConfigProvider struct {
	cfg *AppConfig
}

func (p *appConfigProvider) GetGRPCConfig() GRPCServerConfig {
	return GRPCServerConfig{
		Host:              p.cfg.GRPC.Host,
		Port:              p.cfg.GRPC.Port,
		ReadTimeout:       p.cfg.GRPC.ReadTimeout,
		WriteTimeout:      p.cfg.GRPC.WriteTimeout,
		IdleTimeout:       p.cfg.GRPC.IdleTimeout,
		ReadHeaderTimeout: p.cfg.GRPC.ReadHeaderTimeout,
	}
}

func (p *appConfigProvider) GetHTTPConfig() HTTPServerConfig {
	return HTTPServerConfig{
		Host:              p.cfg.HTTP.Host,
		Port:              p.cfg.HTTP.Port,
		ReadTimeout:       p.cfg.HTTP.ReadTimeout,
		WriteTimeout:      p.cfg.HTTP.WriteTimeout,
		IdleTimeout:       p.cfg.HTTP.IdleTimeout,
		ReadHeaderTimeout: p.cfg.HTTP.ReadHeaderTimeout,
	}
}

func (p *appConfigProvider) GetHealthConfig() HealthServerConfig {
	return HealthServerConfig{
		Host: p.cfg.Health.Host,
		Port: p.cfg.Health.Port,
	}
}

func (p *appConfigProvider) GetGRPCServices() []GRPCService {
	return p.cfg.GRPCServices
}

var (
	singletonConfigProvider ConfigProvider = nil
	onceProvider            sync.Once

	grpcServiceRegistry = map[string]struct {
		AddressEnv string
		Register   RegisterFunc
	}{
		"UserService": {
			AddressEnv: "USER_SERVICE_ENDPOINT",
			Register:   user.RegisterUserServiceHandler,
		},
	}
)

func LoadConfig() (*AppConfig, error) {
	var (
		cfg            AppConfig
		loadedServices []GRPCService
		logger         *logrus.Entry = logrus.WithField("scope", "config")
	)

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}

	for servicePrefix, registrationInfo := range grpcServiceRegistry {
		address := os.Getenv(registrationInfo.AddressEnv)

		if address == "" {
			logger.Debugf("gRPC service address not configured for '%s' (env var: %s), skipping...", servicePrefix, registrationInfo.AddressEnv)

			continue
		}

		if registrationInfo.Register == nil {
			return nil, fmt.Errorf("internal config error: register function is nil for service '%s'", servicePrefix)
		}

		logger.Infof("found gRPC service '%s' at address: %s", servicePrefix, address)

		loadedServices = append(loadedServices, GRPCService{
			Name:     servicePrefix,
			Address:  address,
			Register: registrationInfo.Register,
		})
	}

	if len(loadedServices) == 0 {
		logger.Warn("no gRPC services were configured or found")
	}

	cfg.GRPCServices = loadedServices

	logger.Info("configuration loaded successfully")

	logger.Debugf("AppEnv: '%s'", cfg.AppEnv)
	logger.Debugf("LogLevel: '%s'", cfg.LogLevel)
	logger.Debugf("GRPC Host: %s, Port: %s", cfg.GRPC.Host, cfg.GRPC.Port)
	logger.Debugf("HTTP Host: %s, Port: %s", cfg.HTTP.Host, cfg.HTTP.Port)
	logger.Debugf("Health Host: %s, Port: %s", cfg.Health.Host, cfg.Health.Port)

	for _, svc := range cfg.GRPCServices {
		logger.Debugf("gRPC service: Name='%s', Address='%s'", svc.Name, svc.Address)
	}

	return &cfg, nil
}

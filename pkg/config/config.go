package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/invenlore/core/pkg/logger"
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

type MongoConfig struct {
	URI              string
	DatabaseName     string
	OperationTimeout time.Duration
}

type appConfig struct {
	AppEnv               AppEnv          `env:"APP_ENV" envDefault:"dev"`
	LogLevel             logger.LogLevel `env:"APP_LOG_LEVEL" envDefault:"INFO"`
	ServiceHealthTimeout time.Duration   `env:"SERVICE_HEALTH_TIMEOUT" envDefault:"60s"`

	GRPC struct {
		Host string `env:"HOST" envDefault:"0.0.0.0"`
		Port string `env:"PORT" envDefault:"8080"`
	} `envPrefix:"GRPC_"`

	HTTP struct {
		Host              string        `env:"HOST" envDefault:"0.0.0.0"`
		Port              string        `env:"PORT" envDefault:"8080"`
		ReadTimeout       time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
		WriteTimeout      time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
		IdleTimeout       time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
		ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"5s"`
	} `envPrefix:"HTTP_"`

	Health struct {
		Host string `env:"HOST" envDefault:"0.0.0.0"`
		Port string `env:"PORT" envDefault:"80"`
	} `envPrefix:"HEALTH_"`

	Mongo struct {
		URI              string        `env:"URI,notEmpty"`
		DatabaseName     string        `env:"DATABASE_NAME,notEmpty"`
		OperationTimeout time.Duration `env:"OPERATION_TIMEOUT" envDefault:"10s"`
	} `envPrefix:"MONGO_"`

	GRPCServices []*GRPCService `env:"-"`
}

type AppConfig interface {
	GetGRPCConfig() *GRPCServerConfig
	GetHTTPConfig() *HTTPServerConfig
	GetHealthConfig() *HealthServerConfig
	GetMongoConfig() *MongoConfig
	GetGRPCServices() []*GRPCService
}

func (p *appConfig) GetGRPCConfig() *GRPCServerConfig {
	return &GRPCServerConfig{
		Host: p.GRPC.Host,
		Port: p.GRPC.Port,
	}
}

func (p *appConfig) GetHTTPConfig() *HTTPServerConfig {
	return &HTTPServerConfig{
		Host:              p.HTTP.Host,
		Port:              p.HTTP.Port,
		ReadTimeout:       p.HTTP.ReadTimeout,
		WriteTimeout:      p.HTTP.WriteTimeout,
		IdleTimeout:       p.HTTP.IdleTimeout,
		ReadHeaderTimeout: p.HTTP.ReadHeaderTimeout,
	}
}

func (p *appConfig) GetHealthConfig() *HealthServerConfig {
	return &HealthServerConfig{
		Host: p.Health.Host,
		Port: p.Health.Port,
	}
}

func (p *appConfig) GetMongoConfig() *MongoConfig {
	return &MongoConfig{
		URI:              p.Mongo.URI,
		DatabaseName:     p.Mongo.DatabaseName,
		OperationTimeout: p.Mongo.OperationTimeout,
	}
}

func (p *appConfig) GetGRPCServices() []*GRPCService {
	return p.GRPCServices
}

var (
	once                sync.Once
	configLoadingErr    error
	instance            *appConfig
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

func LoadConfig() (*appConfig, error) {
	var (
		cfg            appConfig
		loadedServices []*GRPCService
		loggerEntry    *logrus.Entry = logrus.WithField("scope", "config")
	)

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}

	logrus.SetLevel(cfg.LogLevel.ToLogrusLevel())

	for servicePrefix, registrationInfo := range grpcServiceRegistry {
		address := os.Getenv(registrationInfo.AddressEnv)

		if address == "" {
			loggerEntry.Debugf("gRPC service address not configured for '%s' (env var: %s), skipping...", servicePrefix, registrationInfo.AddressEnv)

			continue
		}

		if registrationInfo.Register == nil {
			return nil, fmt.Errorf("internal config error: register function is nil for service '%s'", servicePrefix)
		}

		loggerEntry.Infof("found gRPC service '%s' at address: %s", servicePrefix, address)

		loadedServices = append(loadedServices, &GRPCService{
			Name:     servicePrefix,
			Address:  address,
			Register: registrationInfo.Register,
		})
	}

	if len(loadedServices) == 0 {
		loggerEntry.Warn("no gRPC services were configured or found")
	}

	cfg.GRPCServices = loadedServices

	loggerEntry.Info("configuration loaded successfully")

	loggerEntry.Debugf("AppEnv: '%s'", cfg.AppEnv)
	loggerEntry.Debugf("LogLevel: '%s'", cfg.LogLevel)
	loggerEntry.Debugf("GRPC Host: %s, Port: %s", cfg.GRPC.Host, cfg.GRPC.Port)
	loggerEntry.Debugf("HTTP Host: %s, Port: %s", cfg.HTTP.Host, cfg.HTTP.Port)
	loggerEntry.Debugf("Health Host: %s, Port: %s", cfg.Health.Host, cfg.Health.Port)
	loggerEntry.Debugf("MongoDB database: '%s'", cfg.Mongo.DatabaseName)

	for _, svc := range cfg.GRPCServices {
		loggerEntry.Debugf("gRPC service: Name='%s', Address='%s'", svc.Name, svc.Address)
	}

	return &cfg, nil
}

func GetConfig() (AppConfig, error) {
	once.Do(func() {
		instance, configLoadingErr = LoadConfig()
		if configLoadingErr != nil {
			logrus.Errorf("config loading failed: %v", configLoadingErr)
		}
	})

	return instance, configLoadingErr
}

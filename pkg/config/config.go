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
	Host string `env:"HOST" envDefault:"0.0.0.0"`
	Port string `env:"PORT" envDefault:"8080"`
}

type HTTPServerConfig struct {
	Host              string        `env:"HOST" envDefault:"0.0.0.0"`
	Port              string        `env:"PORT" envDefault:"8080"`
	ReadTimeout       time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout      time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"5s"`
}

type HealthServerConfig struct {
	Host              string        `env:"HOST" envDefault:"0.0.0.0"`
	Port              string        `env:"PORT" envDefault:"80"`
	ReadTimeout       time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout      time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"5s"`
}

type MongoConfig struct {
	URI                 string        `env:"URI"`
	DatabaseName        string        `env:"DATABASE_NAME"`
	HealthCheckTimeout  time.Duration `env:"HEALTHCHECK_TIMEOUT" envDefault:"2s"`
	HealthCheckInterval time.Duration `env:"HEALTHCHECK_INTERVAL" envDefault:"10s"`
	OperationTimeout    time.Duration `env:"OPERATION_TIMEOUT" envDefault:"10s"`
}

type AppConfig struct {
	AppEnv               AppEnv          `env:"APP_ENV" envDefault:"dev"`
	LogLevel             logger.LogLevel `env:"APP_LOG_LEVEL" envDefault:"INFO"`
	ServiceHealthTimeout time.Duration   `env:"SERVICE_HEALTH_TIMEOUT" envDefault:"60s"`

	GRPC   GRPCServerConfig   `envPrefix:"GRPC_"`
	HTTP   HTTPServerConfig   `envPrefix:"HTTP_"`
	Health HealthServerConfig `envPrefix:"HEALTH_"`
	Mongo  MongoConfig        `envPrefix:"MONGO_"`

	GRPCServices []*GRPCService `env:"-"`
}

type AppConfigProvider interface {
	GetConfig() *AppConfig
	GetGRPCConfig() *GRPCServerConfig
	GetHTTPConfig() *HTTPServerConfig
	GetHealthConfig() *HealthServerConfig
	GetMongoConfig() *MongoConfig
	GetGRPCServices() []*GRPCService
}

func (p *AppConfig) GetConfig() *AppConfig {
	return p
}

func (p *AppConfig) GetGRPCConfig() *GRPCServerConfig {
	return &p.GRPC
}

func (p *AppConfig) GetHTTPConfig() *HTTPServerConfig {
	return &p.HTTP
}

func (p *AppConfig) GetHealthConfig() *HealthServerConfig {
	return &p.Health
}

func (p *AppConfig) GetMongoConfig() *MongoConfig {
	return &p.Mongo
}

func (p *AppConfig) GetGRPCServices() []*GRPCService {
	return p.GRPCServices
}

var (
	once                sync.Once
	configLoadingErr    error
	instance            *AppConfig
	loggerEntry         = logrus.WithField("scope", "config")
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
		loadedServices []*GRPCService
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

	if cfg.Mongo.DatabaseName != "" {
		loggerEntry.Debugf("MongoDB database: '%s'", cfg.Mongo.DatabaseName)
	}

	for _, svc := range cfg.GRPCServices {
		loggerEntry.Debugf("gRPC service: Name='%s', Address='%s'", svc.Name, svc.Address)
	}

	return &cfg, nil
}

func Config() (AppConfigProvider, error) {
	once.Do(func() {
		instance, configLoadingErr = LoadConfig()
		if configLoadingErr != nil {
			loggerEntry.Errorf("config loading failed: %v", configLoadingErr)
		}
	})

	return instance, configLoadingErr
}

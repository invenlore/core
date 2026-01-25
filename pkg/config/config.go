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
	identity_v1 "github.com/invenlore/proto/pkg/identity/v1"
	media_v1 "github.com/invenlore/proto/pkg/media/v1"
	search_v1 "github.com/invenlore/proto/pkg/search/v1"
	wiki_v1 "github.com/invenlore/proto/pkg/wiki/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type RegisterFunc func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

type RegisterEntry struct {
	HandlerName         string
	HandlerRegisterFunc RegisterFunc
}

type GRPCService struct {
	Name            string
	Address         string
	RegisterEntries []RegisterEntry
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
	URI                      string        `env:"URI"`
	DatabaseName             string        `env:"DATABASE_NAME"`
	HealthCheckTimeout       time.Duration `env:"HEALTHCHECK_TIMEOUT" envDefault:"2s"`
	HealthCheckInterval      time.Duration `env:"HEALTHCHECK_INTERVAL" envDefault:"10s"`
	OperationTimeout         time.Duration `env:"OPERATION_TIMEOUT" envDefault:"10s"`
	MigrationTimeout         time.Duration `env:"MIGRATION_TIMEOUT" envDefault:"15m"`
	MigrationLeaseForTimeout time.Duration `env:"MIGRATION_LEASEFOR_TIMEOUT" envDefault:"30s"`
	MigrationPollInterval    time.Duration `env:"MIGRATION_POLL_INTERVAL" envDefault:"2s"`
	MigrationServiceTimeout  time.Duration `env:"MIGRATION_SERVICE_TIMEOUT" envDefault:"5s"`
}

type AuthConfig struct {
	AccessTokenTTL      time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`
	RefreshTokenTTL     time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
	JWTIssuer           string        `env:"JWT_ISSUER" envDefault:"invenlore.identity"`
	JWTAudience         string        `env:"JWT_AUDIENCE" envDefault:"invenlore.api"`
	JWKSCacheTTL        time.Duration `env:"JWKS_CACHE_TTL" envDefault:"10m"`
	JWTAllowedSkew      time.Duration `env:"JWT_ALLOWED_SKEW" envDefault:"60s"`
	KeyRotationInterval time.Duration `env:"KEY_ROTATION_INTERVAL" envDefault:"168h"`
	KeyRetireAfter      time.Duration `env:"KEY_RETIRE_AFTER" envDefault:"1h"`
}

type AppConfig struct {
	AppEnv               AppEnv          `env:"APP_ENV" envDefault:"dev"`
	LogLevel             logger.LogLevel `env:"APP_LOG_LEVEL" envDefault:"INFO"`
	ServiceHealthTimeout time.Duration   `env:"SERVICE_HEALTH_TIMEOUT" envDefault:"60s"`

	GRPC   GRPCServerConfig   `envPrefix:"GRPC_"`
	HTTP   HTTPServerConfig   `envPrefix:"HTTP_"`
	Health HealthServerConfig `envPrefix:"HEALTH_"`
	Auth   AuthConfig         `envPrefix:"AUTH_"`
	Mongo  MongoConfig        `envPrefix:"MONGO_"`

	GRPCServices []*GRPCService `env:"-"`
}

type AppConfigProvider interface {
	GetConfig() *AppConfig
	GetGRPCConfig() *GRPCServerConfig
	GetHTTPConfig() *HTTPServerConfig
	GetAuthConfig() *AuthConfig
	GetHealthConfig() *HealthServerConfig
	GetMongoConfig() *MongoConfig
	GetGRPCServices() []*GRPCService
}

func (p *AppConfig) GetConfig() *AppConfig {
	return p
}

func (p *AppConfig) GetAuthConfig() *AuthConfig {
	return &p.Auth
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
		AddressEnv      string
		RegisterEntries []RegisterEntry
	}{
		"IdentityService": {
			AddressEnv: "IDENTITY_SERVICE_ENDPOINT",
			RegisterEntries: []RegisterEntry{
				{
					HandlerName:         "IdentityPublicServiceHandler",
					HandlerRegisterFunc: identity_v1.RegisterIdentityPublicServiceHandler,
				}, {
					HandlerName:         "IdentityInternalServiceHandler",
					HandlerRegisterFunc: identity_v1.RegisterIdentityInternalServiceHandler,
				},
			},
		},
		"WikiReadService": {
			AddressEnv: "WIKI_READ_SERVICE_ENDPOINT",
			RegisterEntries: []RegisterEntry{
				{
					HandlerName:         "WikiReadServiceHandler",
					HandlerRegisterFunc: wiki_v1.RegisterWikiReadServiceHandler,
				},
			},
		},
		"WikiWriteService": {
			AddressEnv: "WIKI_WRITE_SERVICE_ENDPOINT",
			RegisterEntries: []RegisterEntry{
				{
					HandlerName:         "WikiWriteServiceHandler",
					HandlerRegisterFunc: wiki_v1.RegisterWikiWriteServiceHandler,
				},
			},
		},
		"MediaService": {
			AddressEnv: "MEDIA_SERVICE_ENDPOINT",
			RegisterEntries: []RegisterEntry{
				{
					HandlerName:         "MediaServiceHandler",
					HandlerRegisterFunc: media_v1.RegisterMediaServiceHandler,
				},
			},
		},
		"SearchService": {
			AddressEnv: "SEARCH_SERVICE_ENDPOINT",
			RegisterEntries: []RegisterEntry{
				{
					HandlerName:         "SearchServiceHandler",
					HandlerRegisterFunc: search_v1.RegisterSearchServiceHandler,
				},
			},
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
			loggerEntry.Debugf(
				"gRPC service address not configured for '%s' (env var: %s), skipping...",
				servicePrefix,
				registrationInfo.AddressEnv,
			)

			continue
		}

		if len(registrationInfo.RegisterEntries) == 0 {
			return nil, fmt.Errorf("internal config error: register function is not specified for service '%s'", servicePrefix)
		} else {
			for _, registerEntry := range registrationInfo.RegisterEntries {
				if registerEntry.HandlerRegisterFunc == nil {
					return nil, fmt.Errorf(
						"internal config error: register function '%s' is nil for service '%s'",
						registerEntry.HandlerName,
						servicePrefix,
					)
				}
			}
		}

		loggerEntry.Infof("found gRPC service '%s' at address: %s", servicePrefix, address)

		loadedServices = append(loadedServices, &GRPCService{
			Name:            servicePrefix,
			Address:         address,
			RegisterEntries: registrationInfo.RegisterEntries,
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

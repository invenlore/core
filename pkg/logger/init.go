package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Level   LogLevel
	Env     string
	Service string
	Version string
}

type defaultFieldsHook struct {
	fields logrus.Fields
}

func (h defaultFieldsHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h defaultFieldsHook) Fire(entry *logrus.Entry) error {
	for key, value := range h.fields {
		if _, exists := entry.Data[key]; !exists && value != "" {
			entry.Data[key] = value
		}
	}

	return nil
}

var (
	initOnce       sync.Once
	defaultHookRef *defaultFieldsHook
)

func Init(cfg Config) {
	logger := logrus.StandardLogger()
	logger.SetFormatter(jsonFormatter())
	logger.SetLevel(cfg.Level.ToLogrusLevel())

	fields := logrus.Fields{
		"service": cfg.Service,
		"version": cfg.Version,
		"env":     cfg.Env,
	}

	initOnce.Do(func() {
		defaultHookRef = &defaultFieldsHook{fields: fields}
		logger.AddHook(defaultHookRef)
	})

	if defaultHookRef != nil {
		defaultHookRef.fields = fields
	}
}

func InitEarlyFromEnv() {
	var cfg Config

	if level := strings.TrimSpace(os.Getenv("APP_LOG_LEVEL")); level != "" {
		cfg.Level = LogLevel(strings.ToUpper(level))
	} else {
		cfg.Level = LogLevelInfo
	}

	cfg.Env = strings.TrimSpace(os.Getenv("APP_ENV"))
	cfg.Service = strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	cfg.Version = strings.TrimSpace(os.Getenv("SERVICE_VERSION"))

	if cfg.Version == "" {
		if version, err := readServiceVersion(); err == nil {
			cfg.Version = version
		}
	}

	if cfg.Service == "" {
		cfg.Service = "unknown"
	}

	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	Init(cfg)
}

func jsonFormatter() *logrus.JSONFormatter {
	return &logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	}
}

func readServiceVersion() (string, error) {
	const maxVersionLength = 128

	path := filepath.FromSlash("/app/version.txt")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", fmt.Errorf("service version file is empty")
	}

	if len(version) > maxVersionLength {
		return "", fmt.Errorf("service version length exceeds limit")
	}

	return version, nil
}

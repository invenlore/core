package config

import "strings"

// "dev" | "prod"
type AppEnv string

const (
	AppEnvDevelopment AppEnv = "dev"
	AppEnvProduction  AppEnv = "prod"
)

func (s AppEnv) String() string {
	return string(s)
}

func (s *AppEnv) UnmarshalText(text []byte) error {
	tt := strings.ToLower(string(text))
	*s = AppEnv(tt)

	return nil
}

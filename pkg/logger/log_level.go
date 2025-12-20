package logger

import (
	"strings"

	"github.com/sirupsen/logrus"
)

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelTrace LogLevel = "TRACE"
	LogLevelError LogLevel = "ERROR"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelFatal LogLevel = "FATAL"
	LogLevelPanic LogLevel = "PANIC"
)

func (s LogLevel) String() string {
	return string(s)
}

func (s *LogLevel) UnmarshalText(text []byte) error {
	tt := strings.ToUpper(string(text))
	*s = LogLevel(tt)

	return nil
}

func (s LogLevel) ToLogrusLevel() logrus.Level {
	switch s {
	case LogLevelInfo:
		return logrus.InfoLevel
	case LogLevelDebug:
		return logrus.DebugLevel
	case LogLevelTrace:
		return logrus.TraceLevel
	case LogLevelError:
		return logrus.ErrorLevel
	case LogLevelWarn:
		return logrus.WarnLevel
	case LogLevelFatal:
		return logrus.FatalLevel
	case LogLevelPanic:
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}

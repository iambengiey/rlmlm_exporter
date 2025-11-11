package promslog

import (
	"io"
	"os"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type Config struct {
	Format string
	Level  string
	Writer io.Writer
}

func New(cfg *Config) log.Logger {
	if cfg == nil {
		cfg = &Config{}
	}
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	format := strings.ToLower(cfg.Format)
	if format == "" {
		format = "logfmt"
	}
	lvl := level.ParseLevel(cfg.Level)
	base := log.NewStdLogger(writer, lvl, format)
	return base
}

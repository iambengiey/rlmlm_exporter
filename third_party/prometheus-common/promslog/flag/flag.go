package flag

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/promslog"
)

func AddFlags(app *kingpin.Application, cfg *promslog.Config) {
	if cfg == nil {
		return
	}
	formatDefault := cfg.Format
	if formatDefault == "" {
		formatDefault = "logfmt"
	}
	levelDefault := cfg.Level
	if levelDefault == "" {
		levelDefault = "info"
	}
	formatPtr := app.Flag("log.format", "Output format of log messages (logfmt or json).").Default(formatDefault).String()
	levelPtr := app.Flag("log.level", "Minimum accepted logging level.").Default(levelDefault).String()
	app.AfterParse(func() {
		if formatPtr != nil {
			cfg.Format = *formatPtr
		}
		if levelPtr != nil {
			cfg.Level = *levelPtr
		}
	})
}

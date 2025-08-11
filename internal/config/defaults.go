package config

import (
	"log/slog"
)

//nolint:gochecknoglobals
var Defaults = Config{
	ConfigFile:  "config.yaml",
	BufferSize:  1000,
	WorkerCount: 0,
	Preset:      "simple",
	Debug:       Debug{},
	Log: Log{
		Format: "console",
		Level:  slog.LevelInfo,
	},
	Web: Web{
		ListenAddress: ":4040",
	},
	Syslog: Syslog{
		ListenAddress: "udp://[::]:8514",
	},
}

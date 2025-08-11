package config

import (
	"log/slog"
	"runtime"
)

//nolint:gochecknoglobals
var Defaults = Config{
	ConfigFile:  "config.yaml",
	BufferSize:  1000,
	WorkerCount: uint(runtime.NumCPU()),
	Preset:      "simple",
	Debug: Debug{
		ListenAddress: ":9001",
	},
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

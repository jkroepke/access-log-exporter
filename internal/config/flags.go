package config

import (
	"flag"
)

//goland:noinspection GoMixedReceiverTypes
func (c *Config) flagSet(flagSet *flag.FlagSet) {
	flagSet.String(
		"config",
		"",
		"path to one .yaml config file",
	)

	flagSet.Bool(
		"version",
		false,
		"show version",
	)

	flagSet.BoolVar(
		&c.VerifyConfig,
		"verify-config",
		c.VerifyConfig,
		"Enable this flag to check config file loads, then exit",
	)

	flagSet.UintVar(
		&c.BufferSize,
		"buffer-size",
		lookupEnvOrDefault("buffer_size", c.BufferSize),
		"Size of the buffer for syslog messages. Default is 1000. Set to 0 to disable buffering.",
	)

	flagSet.UintVar(
		&c.WorkerCount,
		"worker",
		lookupEnvOrDefault("worker", c.WorkerCount),
		"Number of workers to process syslog messages. Default is number of CPU cores.",
	)

	flagSet.StringVar(
		&c.Preset,
		"preset",
		lookupEnvOrDefault("preset", c.Preset),
		"Preset configuration to use. Available presets: simple, simple_upstream, all. Custom presets can be defined via config file. Default is simple.",
	)

	c.flagSetDebug(flagSet)
	c.flagSetWeb(flagSet)
	c.flagSetSyslog(flagSet)
}

//goland:noinspection GoMixedReceiverTypes
func (c *Config) flagSetDebug(flagSet *flag.FlagSet) {
	flagSet.BoolVar(
		&c.Debug.Pprof,
		"debug.pprof",
		lookupEnvOrDefault("debug.pprof", c.Debug.Pprof),
		"Enables go profiling endpoint. This should be never exposed.",
	)
	flagSet.StringVar(
		&c.Debug.ListenAddress,
		"debug.listen",
		lookupEnvOrDefault("debug.listen", c.Debug.ListenAddress),
		"listen address for go profiling endpoint",
	)
}

//goland:noinspection GoMixedReceiverTypes
func (c *Config) flagSetWeb(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.Web.ListenAddress,
		"web.listen-address",
		lookupEnvOrDefault("web.listen-address", c.Web.ListenAddress),
		"Addresses on which to expose metrics. Examples: `:9100` or `[::1]:9100` for http, `vsock://:9100` for vsock.",
	)
	flagSet.StringVar(
		&c.Web.ConfigFile,
		"web.config",
		lookupEnvOrDefault("web.config", c.Web.ConfigFile),
		"Path to configuration file that can enable TLS or authentication. See: https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md",
	)
}

//goland:noinspection GoMixedReceiverTypes
func (c *Config) flagSetSyslog(flagSet *flag.FlagSet) {
	flagSet.StringVar(
		&c.Syslog.ListenAddress,
		"syslog.listen-address",
		lookupEnvOrDefault("syslog.listen-address", c.Syslog.ListenAddress),
		"Addresses on which to expose syslog. Examples: udp://0.0.0.0:8514, tcp://0.0.0.0:8514, unix:///path/to/socket.",
	)
}

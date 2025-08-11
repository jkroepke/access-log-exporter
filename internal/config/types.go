package config

import (
	"encoding/json"
	"log/slog"
	"regexp"

	"github.com/jkroepke/access-log-exporter/internal/config/types"
)

type Config struct {
	VerifyConfig bool

	ConfigFile string `yaml:"config"`

	Web    Web    `yaml:"web"`
	Debug  Debug  `yaml:"debug"`
	Log    Log    `yaml:"log"`
	Syslog Syslog `yaml:"syslog"`

	WorkerCount uint    `yaml:"workerCount"`
	BufferSize  uint    `yaml:"bufferSize"`
	Preset      string  `yaml:"preset"`
	Presets     Presets `yaml:"presets"`
}

type Log struct {
	Format string     `yaml:"format"`
	Level  slog.Level `yaml:"level"`
}

type Syslog struct {
	ListenAddress string `yaml:"listenAddress"`
}

type Debug struct {
	ListenAddress string `json:"listen-address" yaml:"listen-address"`
	Pprof         bool   `json:"pprof"  yaml:"pprof"`
}

type Web struct {
	ListenAddress string `json:"listen-address" yaml:"listen-address"`
	ConfigFile    string `json:"config" yaml:"config"`
}

type Presets map[string]Preset

type Preset struct {
	Metrics []Metric `yaml:"metrics"`
}

type Metric struct {
	Name        string             `yaml:"name"`
	Type        string             `yaml:"type"`
	Help        string             `yaml:"help"`
	ConstLabels map[string]string  `yaml:"constLabels"`
	Buckets     types.Float64Slice `yaml:"buckets,omitempty"`
	ValueIndex  *uint              `yaml:"valueIndex,omitempty"`
	Math        Math               `yaml:"math"`
	Upstream    Upstream           `yaml:"upstream"`
	Labels      []Label            `yaml:"labels"`
}

type Math struct {
	Enabled bool    `yaml:"enabled"`
	Mul     float64 `yaml:"mul"`
	Div     float64 `yaml:"div"`
}

type Upstream struct {
	Enabled       bool     `yaml:"enabled"`
	AddrLineIndex uint     `yaml:"addrLineIndex"`
	Excludes      []string `yaml:"excludes"`
	Label         bool     `yaml:"label"`
}

type Label struct {
	Name         string        `yaml:"name"`
	LineIndex    uint          `yaml:"lineIndex"`
	Replacements []Replacement `yaml:"replacements,omitempty"`
	UserAgent    bool          `yaml:"userAgent"`
}

type Replacement struct {
	Regexp      regexp.Regexp `yaml:"regexp"`
	Replacement string        `yaml:"replacement"`
}

//goland:noinspection GoMixedReceiverTypes
func (c Config) String() string {
	jsonString, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return string(jsonString)
}

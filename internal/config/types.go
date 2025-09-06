package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jkroepke/access-log-exporter/internal/config/types"
	"go.yaml.in/yaml/v4"
)

var ErrEmptyConfigFile = errors.New("configuration file is empty")

type Config struct {
	Presets      Presets `json:"presets"     yaml:"presets"`
	Nginx        Nginx   `json:"nginx"       yaml:"nginx"`
	Web          Web     `json:"web"         yaml:"web"`
	ConfigFile   string  `json:"config"      yaml:"config"`
	Syslog       Syslog  `json:"syslog"      yaml:"syslog"`
	Preset       string  `json:"preset"      yaml:"preset"`
	Log          Log     `json:"log"         yaml:"log"`
	WorkerCount  int     `json:"workerCount" yaml:"workerCount"`
	BufferSize   uint    `json:"bufferSize"  yaml:"bufferSize"`
	Debug        Debug   `json:"debug"       yaml:"debug"`
	VerifyConfig bool    `json:"-"`
}

type Log struct {
	Format string     `json:"format" yaml:"format"`
	Level  slog.Level `json:"level"  yaml:"level"`
}

type Syslog struct {
	ListenAddress string `json:"listenAddress" yaml:"listenAddress"`
}

type Debug struct {
	Enable bool `json:"enable" yaml:"enable"`
}

type Web struct {
	ListenAddress string `json:"listenAddress" yaml:"listenAddress"`
}

type Presets map[string]Preset

type Preset struct {
	Metrics []Metric `json:"metrics" yaml:"metrics"`
}

type Metric struct {
	ConstLabels  map[string]string  `json:"constLabels"            yaml:"constLabels"`
	ValueIndex   *uint              `json:"valueIndex,omitempty"   yaml:"valueIndex,omitempty"`
	Name         string             `json:"name"                   yaml:"name"`
	Type         string             `json:"type"                   yaml:"type"`
	Help         string             `json:"help"                   yaml:"help"`
	Buckets      types.Float64Slice `json:"buckets,omitempty"      yaml:"buckets,omitempty"`
	Labels       []Label            `json:"labels"                 yaml:"labels"`
	Replacements []Replacement      `json:"replacements,omitempty" yaml:"replacements,omitempty"`
	Upstream     Upstream           `json:"upstream"               yaml:"upstream"`
	Math         Math               `json:"math"                   yaml:"math"`
}

type Math struct {
	Enabled bool    `json:"enabled" yaml:"enabled"`
	Mul     float64 `json:"mul"     yaml:"mul"`
	Div     float64 `json:"div"     yaml:"div"`
}

type Upstream struct {
	Excludes      []string `json:"excludes"      yaml:"excludes"`
	AddrLineIndex uint     `json:"addrLineIndex" yaml:"addrLineIndex"`
	Enabled       bool     `json:"enabled"       yaml:"enabled"`
	Label         bool     `json:"label"         yaml:"label"`
}

type Label struct {
	Name         string        `json:"name"                   yaml:"name"`
	Replacements []Replacement `json:"replacements,omitempty" yaml:"replacements,omitempty"`
	LineIndex    uint          `json:"lineIndex"              yaml:"lineIndex"`
	UserAgent    bool          `json:"userAgent"              yaml:"userAgent"`
}

type Replacement struct {
	String         *string           `json:"string,omitempty" yaml:"string,omitempty"`
	Regexp         *regexp.Regexp    `json:"regexp,omitempty" yaml:"regexp,omitempty"`
	StringReplacer *strings.Replacer `json:"-"                yaml:"-"`
	Replacement    string            `json:"replacement"      yaml:"replacement"`
}

type Nginx struct {
	ScrapeURL types.URL `json:"scrapeUri" yaml:"scrapeUri"`
}

//goland:noinspection GoMixedReceiverTypes
func (c Config) String() string {
	jsonString, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return string(jsonString)
}

func (r *Replacement) UnmarshalYAML(data *yaml.Node) error {
	type Alias Replacement

	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := data.Decode(&aux); err != nil {
		return err //nolint:wrapcheck
	}

	if r.Regexp != nil && r.String != nil {
		return errors.New("replacement can not have both regexp and string")
	}

	if r.String != nil {
		r.StringReplacer = strings.NewReplacer(*r.String, r.Replacement)
	}

	return nil
}

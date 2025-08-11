package config

import (
	"fmt"
	"regexp"
)

// RelabelConfig is a struct describing a single re-labeling configuration for taking
// over label values from an access log line into a Prometheus metric
type RelabelConfig struct {
	TargetLabel string `yaml:"target_label"`
	LineIndex   uint   `yaml:"line_index"`
	Whitelist   []string
	Matches     []RelabelValueMatch
	Split       int
	Separator   string
	OnlyCounter bool `yaml:"only_counter"`

	WhitelistExists bool
	WhitelistMap    map[string]interface{}
}

// RelabelValueMatch describes a single label match statement
type RelabelValueMatch struct {
	RegexpString string `yaml:"regexp"`
	Replacement  string

	CompiledRegexp *regexp.Regexp
}

// Compile compiles expressions and lookup tables for efficient later use
func (c *RelabelConfig) Compile() error {
	c.WhitelistMap = make(map[string]interface{})
	c.WhitelistExists = len(c.Whitelist) > 0

	for i := range c.Whitelist {
		c.WhitelistMap[c.Whitelist[i]] = nil
	}

	for i := range c.Matches {
		if c.Matches[i].RegexpString != "" {
			r, err := regexp.Compile(c.Matches[i].RegexpString)
			if err != nil {
				return fmt.Errorf("could not compile regexp '%s': %s", c.Matches[i].RegexpString, err.Error())
			}

			c.Matches[i].CompiledRegexp = r
		}
	}

	return nil
}

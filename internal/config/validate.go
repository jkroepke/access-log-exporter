package config

import (
	"fmt"
)

// Validate validates the config.
func Validate(conf Config) error {
	_, ok := conf.Presets[conf.Preset]
	if !ok {
		return fmt.Errorf("preset '%s' not found in configuration", conf.Preset)
	}

	return nil
}

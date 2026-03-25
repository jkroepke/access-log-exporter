package config

import (
	"errors"
	"fmt"
)

// Validate validates the config.
func Validate(conf Config) error {
	_, ok := conf.Presets[conf.Preset]
	if !ok {
		return fmt.Errorf("preset '%s' not found in configuration", conf.Preset)
	}

	return validateTLS(conf)
}

// validateTLS validates TLS configuration.
func validateTLS(conf Config) error {
	certSet := conf.Web.TLSCertFile != ""
	keySet := conf.Web.TLSKeyFile != ""

	if certSet != keySet {
		return errors.New("both TLS certificate and key files must be set to enable TLS")
	}

	return nil
}

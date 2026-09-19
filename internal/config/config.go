package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v4"
)

var ErrVersion = errors.New("flag: version requested")

// New loads the configuration from configuration files, environment variables, and command line arguments in that order.
//
//goland:noinspection GoMixedReceiverTypes
func New(args []string, writer io.Writer) (Config, error) {
	config := Defaults

	if !lookupVersionOrHelpArgument(args) {
		configFilePath := lookupConfigArgument(args)
		if err := config.ReadFromConfigFile(configFilePath); err != nil {
			if errors.Is(err, io.EOF) {
				err = ErrEmptyConfigFile
			}

			return Config{}, err
		}
	}

	if err := config.ReadFromFlagAndEnvironment(args, writer); err != nil {
		return Config{}, err
	}

	return config, nil
}

// ReadFromConfigFile reads the configuration from a configuration file.
//
//goland:noinspection GoMixedReceiverTypes
func (c *Config) ReadFromConfigFile(configFilePath string) error {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("error opening config file %s: %w", configFilePath, err)
	}

	defer func() {
		_ = configFile.Close()
	}()

	decoder := yaml.NewDecoder(configFile)
	decoder.KnownFields(true)

	// Load the config file
	if err = decoder.Decode(c); err != nil {
		return fmt.Errorf("error decoding config file %s: %w", configFilePath, err)
	}

	return nil
}

// ReadFromFlagAndEnvironment reads configuration overrides from environment variables and command line arguments.
//
// Environment variables are applied before command line arguments so command line arguments keep the highest priority.
//
//goland:noinspection GoMixedReceiverTypes
func (c *Config) ReadFromFlagAndEnvironment(args []string, writer io.Writer) error {
	flagSet := c.newFlagSet(writer)

	if err := applyEnvironment(flagSet); err != nil {
		return fmt.Errorf("error parsing environment variables: %w", err)
	}

	if err := flagSet.Parse(args[1:]); err != nil {
		return fmt.Errorf("error parsing command line arguments: %w", err)
	}

	if flagSet.NArg() != 0 {
		return errors.New("error parsing command line arguments: positional arguments are not supported")
	}

	if flagSet.Lookup("version").Value.String() == "true" {
		return ErrVersion
	}

	return nil
}

func (c *Config) newFlagSet(writer io.Writer) *flag.FlagSet {
	flagSet := flag.NewFlagSet("access-log-exporter", flag.ContinueOnError)
	flagSet.SetOutput(writer)
	flagSet.Usage = func() {
		_, _ = fmt.Fprint(flagSet.Output(), "Documentation available at https://github.com/jkroepke/access-log-exporter/wiki\r\n\r\n")
		_, _ = fmt.Fprint(flagSet.Output(), "Usage of access-log-exporter:\r\n\r\n")
		// --help should display options with double dash
		flagSet.VisitAll(func(configurationFlag *flag.Flag) {
			configurationFlag.Name = "-" + configurationFlag.Name
		})
		flagSet.PrintDefaults()
	}

	c.flagSet(flagSet)

	flagSet.VisitAll(func(configurationFlag *flag.Flag) {
		if configurationFlag.Name == "version" {
			return
		}

		configurationFlag.Usage += fmt.Sprintf(" (env: %s)", getEnvironmentVariableByFlagName(configurationFlag.Name))
	})

	return flagSet
}

func lookupConfigArgument(args []string) string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "--config") {
			continue
		}

		if configPath, ok := strings.CutPrefix(arg, "--config="); ok {
			return configPath
		}

		// check if the argument is --config without value and look for the next argument
		if len(args) > i+1 {
			return args[i+1]
		}
	}

	if configFilePath, ok := os.LookupEnv(getEnvironmentVariableByFlagName("config")); ok {
		return configFilePath
	}

	defaultConfigFilePath := "config.yaml"

	if koDataPath, ok := os.LookupEnv("KO_DATA_PATH"); ok {
		// If KO_DATA_PATH is set, use it as the config file path
		defaultConfigFilePath = koDataPath + "/config.yaml"
	}

	return defaultConfigFilePath
}

func lookupVersionOrHelpArgument(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "-help":
			return true
		case "-v", "--version":
			return true
		}
	}

	return false
}

func applyEnvironment(flagSet *flag.FlagSet) error {
	var environmentError error

	flagSet.VisitAll(func(configurationFlag *flag.Flag) {
		if environmentError != nil || configurationFlag.Name == "version" {
			return
		}

		environmentVariable := getEnvironmentVariableByFlagName(configurationFlag.Name)
		if value, ok := os.LookupEnv(environmentVariable); ok {
			environmentError = setEnvironmentValue(configurationFlag, environmentVariable, value)

			return
		}

		if legacyEnvironmentVariable, ok := legacyEnvironmentVariableByFlagName(configurationFlag.Name); ok {
			if value, exists := os.LookupEnv(legacyEnvironmentVariable); exists {
				environmentError = setEnvironmentValue(configurationFlag, legacyEnvironmentVariable, value)
			}
		}
	})

	return environmentError
}

func setEnvironmentValue(configurationFlag *flag.Flag, name, value string) error {
	if getter, ok := configurationFlag.Value.(flag.Getter); ok {
		if _, isBoolean := getter.Get().(bool); isBoolean && value != "true" && value != "false" {
			return fmt.Errorf("invalid value for environment variable %s: use true or false", name)
		}
	}

	if err := configurationFlag.Value.Set(value); err != nil {
		return fmt.Errorf("invalid value for environment variable %s: %w", name, err)
	}

	return nil
}

func legacyEnvironmentVariableByFlagName(flagName string) (string, bool) {
	if flagName == "buffer-size" {
		return "CONFIG_BUFFER_SIZE", true
	}

	return "", false
}

// getEnvironmentVariableByFlagName converts a flag name to an environment variable name.
// It replaces all dots with underscores and all dashes with double underscores.
// It also converts the flag name to uppercase.
func getEnvironmentVariableByFlagName(flagName string) string {
	if flagName == "config" {
		return "CONFIG_FILE"
	}

	return "CONFIG_" + strings.ReplaceAll(strings.ReplaceAll(strings.ToUpper(flagName), ".", "_"), "-", "__")
}

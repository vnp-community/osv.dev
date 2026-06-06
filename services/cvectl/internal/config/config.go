// Package config manages cvectl configuration (~/.cvectl/config.yaml).
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	defaultServerURL = "https://api.osv.dev"
	configFileName   = "config"
	configFileType   = "yaml"
)

// Config holds the cvectl configuration.
type Config struct {
	Server       string `mapstructure:"server"`
	APIKey       string `mapstructure:"api_key"`
	OutputFormat string `mapstructure:"output_format"` // table, json, yaml
}

// Load reads config from ~/.cvectl/config.yaml with env overrides.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	viper.SetConfigName(configFileName)
	viper.SetConfigType(configFileType)
	viper.AddConfigPath(filepath.Join(home, ".cvectl"))
	viper.AddConfigPath(".")

	// Env overrides
	viper.SetEnvPrefix("CVECTL")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("server", defaultServerURL)
	viper.SetDefault("output_format", "table")

	if err := viper.ReadInConfig(); err != nil {
		// Config file not required — use defaults + env
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// ConfigDir returns the path to the cvectl config directory.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cvectl")
}

// EnsureConfigDir creates ~/.cvectl/ if it doesn't exist.
func EnsureConfigDir() error {
	dir := ConfigDir()
	return os.MkdirAll(dir, 0o700)
}

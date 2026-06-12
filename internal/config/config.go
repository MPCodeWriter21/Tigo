// Package config provides the configuration structure for the Tigo application.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TigoConfig struct {
	DefaultPriority int    `yaml:"default_priority"`  // 50
	SortBy          string `yaml:"sort_by"`           // id, priority, due-date, title
	ShowClosed      bool   `yaml:"show_closed"`       // true or false
	FrameStyle      string `yaml:"frame_style"`       // round, double, single
	DueColorEnabled bool   `yaml:"due_color_enabled"` // true or false (default true)
}

// DefaultConfig returns a TigoConfig struct with default values for the Tigo application.
func DefaultConfig() *TigoConfig {
	return &TigoConfig{
		DefaultPriority: 50,
		SortBy:          "id",
		ShowClosed:      false,
		FrameStyle:      "round",
		DueColorEnabled: true,
	}
}

// DefaultTigoDir returns the default directory for Tigo data (e.g. ~/.local/share/tigo).
func DefaultTigoDir() (string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("get user home dir: " + err.Error())
	}
	return filepath.Join(userHomeDir, ".local", "share", "tigo"), nil
}

// UserConfigPath returns the path to the user config file
// Usually `~/.config/tigo/config.yaml` on Unix and `%APPDATA%\tigo\config.yaml` on Windows
func UserConfigPath() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "tigo", "config.yaml"), nil
}

// LoadConfigFromPath loads the Tigo configuration from a YAML file to the provided TigoConfig struct.
// It returns an error if the file cannot be read or if the values are invalid.
func LoadConfigFromPath(configPath string, cfg *TigoConfig) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Unmarshal *onto* the default config so missing fields stay default
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Make sure the values are valid
	if cfg.DefaultPriority < 0 {
		return errors.New("default_priority must be a non-negative integer")
	}
	if cfg.SortBy != "id" && cfg.SortBy != "priority" && cfg.SortBy != "due-date" && cfg.SortBy != "title" {
		return errors.New("sort_by must be one of: id, priority, due-date, title")
	}
	if cfg.FrameStyle != "round" && cfg.FrameStyle != "double" && cfg.FrameStyle != "single" {
		return errors.New("frame_style must be one of: round, double, single")
	}

	return nil
}

// LoadConfig loads the Tigo configuration from the default config path and returns a TigoConfig struct.
// Order of precedence for config loading:
// 1. User config directory
// 2. Local tigo directory in the current root directory (e.g. ./.tigo/config.yaml - overrides user config if it exists)
// 3. Default tigo directory (e.g. ~/.local/share/tigo/config.yaml - only loaded if no local config's found)
// If no config file is found, it returns a TigoConfig struct with default values.
func LoadConfig(tigoRoot string) (*TigoConfig, error) {
	cfg := DefaultConfig()

	// Load config from user config directory if it exists
	userConfigPath, err := UserConfigPath()
	if err != nil {
		return nil, fmt.Errorf("get user config path: %w", err)
	}
	if _, err := os.Stat(userConfigPath); err == nil {
		err = LoadConfigFromPath(userConfigPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("load user config from %s: %w", userConfigPath, err)
		}
	}

	// Look for tigo directory in the current root directory and load config from there if it exists
	localConfigPath := filepath.Join(tigoRoot, "config.yaml")
	if _, err := os.Stat(localConfigPath); err == nil {
		err = LoadConfigFromPath(localConfigPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("load local config from `%s`: %w", localConfigPath, err)
		}
		return cfg, nil
	}

	// If no local config is found, check for config in the default tigo directory
	defaultTigoDir, err := DefaultTigoDir()
	if err != nil {
		return nil, fmt.Errorf("get default tigo dir: %w", err)
	}
	defaultConfigPath := filepath.Join(defaultTigoDir, "config.yaml")
	if _, err := os.Stat(defaultConfigPath); err == nil {
		err = LoadConfigFromPath(defaultConfigPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("load default config from %s: %w", defaultConfigPath, err)
		}
		return cfg, nil
	}

	return cfg, nil
}

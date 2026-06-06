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
	DefaultPriority int    `yaml:"default_priority"` // 50
	SortBy          string `yaml:"sort_by"`          // id, priority, due-date, title
	ShowClosed      bool   `yaml:"show_closed"`      // true or false
	FrameStyle      string `yaml:"frame_style"`      // round, double, single
}

// DefaultConfig returns a TigoConfig struct with default values for the Tigo application.
func DefaultConfig() *TigoConfig {
	return &TigoConfig{
		DefaultPriority: 50,
		SortBy:          "id",
		ShowClosed:      false,
		FrameStyle:      "round",
	}
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
func LoadConfig() (*TigoConfig, error) {
	cfg := DefaultConfig()

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return nil, errors.New("get user config dir: " + err.Error())
	}
	// Load config from user config directory if it exists
	userConfigPath := filepath.Join(userConfigDir, "tigo", "config.yaml")
	if _, err := os.Stat(userConfigPath); err == nil {
		err = LoadConfigFromPath(userConfigPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("load user config from %s: %w", userConfigPath, err)
		}
	}

	// Look for tigo directory in the current working directory and load config from there if it exists
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.New("get current working directory: " + err.Error())
	}
	localConfigPath := filepath.Join(cwd, ".tigo", "config.yaml")
	if _, err := os.Stat(localConfigPath); err == nil {
		err = LoadConfigFromPath(localConfigPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("load local config from %s: %w", localConfigPath, err)
		}
		return cfg, nil
	}

	// If no local config is found, check for config in the default tigo directory
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.New("get user home dir: " + err.Error())
	}
	defaultTigoDir := filepath.Join(userHomeDir, ".local", "share", "tigo")
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

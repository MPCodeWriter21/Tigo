package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultPriority != 50 {
		t.Errorf("DefaultPriority = %d; want 50", cfg.DefaultPriority)
	}
	if cfg.SortBy != "id" {
		t.Errorf("SortBy = %q; want %q", cfg.SortBy, "id")
	}
	if cfg.ShowClosed != false {
		t.Errorf("ShowClosed = %t; want false", cfg.ShowClosed)
	}
	if cfg.FrameStyle != "round" {
		t.Errorf("FrameStyle = %q; want %q", cfg.FrameStyle, "round")
	}
	if cfg.DueColorEnabled != true {
		t.Errorf("DueColorEnabled = %t; want true", cfg.DueColorEnabled)
	}
}

func TestLoadConfigFromPath(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    *TigoConfig
		wantErr bool
	}{
		{
			name: "all fields valid",
			yaml: `default_priority: 70
sort_by: priority
show_closed: true
frame_style: single`,
			want: &TigoConfig{
				DefaultPriority: 70,
				SortBy:          "priority",
				ShowClosed:      true,
				FrameStyle:      "single",
				DueColorEnabled: true,
			},
		},
		{
			name: "partial override – keep defaults",
			yaml: `sort_by: due-date`,
			want: &TigoConfig{
				DefaultPriority: 50,
				SortBy:          "due-date",
				ShowClosed:      false,
				FrameStyle:      "round",
				DueColorEnabled: true,
			},
		},
		{
			name: "empty file – use all defaults",
			yaml: ``,
			want: DefaultConfig(),
		},
		{
			name:    "invalid YAML",
			yaml:    `{{{`,
			wantErr: true,
		},
		{
			name:    "negative priority",
			yaml:    `default_priority: -5`,
			wantErr: true,
		},
		{
			name:    "invalid sort_by",
			yaml:    `sort_by: invalid`,
			wantErr: true,
		},
		{
			name:    "invalid frame_style",
			yaml:    `frame_style: fancy`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			os.WriteFile(path, []byte(tt.yaml), 0644)

			cfg := DefaultConfig()
			err := LoadConfigFromPath(path, cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil(%s)", tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error(%s): %v", tt.name, err)
			}

			if cfg.DefaultPriority != tt.want.DefaultPriority {
				t.Errorf("(%s): DefaultPriority = %d; want %d", tt.name, cfg.DefaultPriority, tt.want.DefaultPriority)
			}
			if cfg.SortBy != tt.want.SortBy {
				t.Errorf("(%s): SortBy = %q; want %q", tt.name, cfg.SortBy, tt.want.SortBy)
			}
			if cfg.ShowClosed != tt.want.ShowClosed {
				t.Errorf("(%s): ShowClosed = %t; want %t", tt.name, cfg.ShowClosed, tt.want.ShowClosed)
			}
			if cfg.FrameStyle != tt.want.FrameStyle {
				t.Errorf("(%s): FrameStyle = %q; want %q", tt.name, cfg.FrameStyle, tt.want.FrameStyle)
			}
			if cfg.DueColorEnabled != tt.want.DueColorEnabled {
				t.Errorf("(%s): DueColorEnabled = %t; want %t", tt.name, cfg.DueColorEnabled, tt.want.DueColorEnabled)
			}
		})
	}
}

func TestLoadConfigFromPath_FileNotFound(t *testing.T) {
	err := LoadConfigFromPath("/nonexistent/path.yaml", nil)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary home‑like structure
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, ".config"))
	t.Setenv("USERPROFILE", home)

	// Populate user config directory (~/.config/tigo/config.yaml)
	userConfigDir := filepath.Join(home, ".config", "tigo")
	os.MkdirAll(userConfigDir, 0755)
	os.WriteFile(filepath.Join(userConfigDir, "config.yaml"),
		[]byte(`sort_by: title
frame_style: double`), 0644)

	// Populate local .tigo/config.yaml in the current directory
	cwd := t.TempDir()
	os.MkdirAll(filepath.Join(cwd, ".tigo"), 0755)
	os.WriteFile(filepath.Join(cwd, ".tigo", "config.yaml"),
		[]byte(`sort_by: priority
show_closed: true`), 0644)

	// Populate the “default” fallback directory (~/.local/share/tigo/config.yaml)
	os.MkdirAll(filepath.Join(home, ".local", "share", "tigo"), 0755)
	os.WriteFile(filepath.Join(home, ".local", "share", "tigo", "config.yaml"),
		[]byte(`default_priority: 70`), 0644)

	// Save original working directory and restore after test
	origCwd, _ := os.Getwd()
	defer os.Chdir(origCwd)
	os.Chdir(cwd)

	cfg, err := LoadConfig(filepath.Join(cwd, ".tigo"))
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	// Expected precedence: local .tigo/config.yaml overrides the user one.
	// Default fallback directory is ignored because we already have a local config.
	want := &TigoConfig{
		DefaultPriority: 50,         // not overridden locally
		SortBy:          "priority", // from local
		ShowClosed:      true,       // from local
		FrameStyle:      "double",   // from user (still inherited because local doesn't set it)
		DueColorEnabled: true,       // default
	}

	if cfg.DefaultPriority != want.DefaultPriority {
		t.Errorf("DefaultPriority = %d; want %d", cfg.DefaultPriority, want.DefaultPriority)
	}
	if cfg.SortBy != want.SortBy {
		t.Errorf("SortBy = %q; want %q", cfg.SortBy, want.SortBy)
	}
	if cfg.ShowClosed != want.ShowClosed {
		t.Errorf("ShowClosed = %t; want %t", cfg.ShowClosed, want.ShowClosed)
	}
	if cfg.FrameStyle != want.FrameStyle {
		t.Errorf("FrameStyle = %q; want %q", cfg.FrameStyle, want.FrameStyle)
	}

	// Test that without any config files, defaults are returned.
	t.Run("no configs", func(t *testing.T) {
		origCwd, _ := os.Getwd()
		defer os.Chdir(origCwd)
		emptyHome := t.TempDir()
		t.Setenv("HOME", emptyHome)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(emptyHome, ".config"))
		t.Setenv("APPDATA", filepath.Join(emptyHome, ".config"))
		t.Setenv("USERPROFILE", emptyHome)
		emptyCwd := t.TempDir()
		os.Chdir(emptyCwd)
		cfg, err := LoadConfig(filepath.Join(emptyCwd, ".tigo"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		def := DefaultConfig()
		if *cfg != *def {
			t.Errorf("got %+v, want default %+v", *cfg, def)
		}
	})
}

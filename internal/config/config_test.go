package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJioTVConfig_Load(t *testing.T) {
	// Create a temporary config file for testing
	tmpFile, err := os.CreateTemp("", "jiotv_go_test_*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	content := []byte("epg: true\ndebug: true\ntitle: TestTitle\n")
	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	tests := []struct {
		name    string
		c       *JioTVConfig
		args    struct{ filename string }
		wantErr bool
	}{
		{
			name:    "Load from valid file",
			c:       &JioTVConfig{},
			args:    struct{ filename string }{filename: tmpFile.Name()},
			wantErr: false,
		},
		{
			name:    "Load from non-existent file",
			c:       &JioTVConfig{},
			args:    struct{ filename string }{filename: "nonexistent.yml"},
			wantErr: true,
		},
		{
			name:    "Load from environment variables",
			c:       &JioTVConfig{},
			args:    struct{ filename string }{filename: ""},
			wantErr: false,
		},
	}
	os.Setenv("JIOTV_EPG", "true")
	defer os.Unsetenv("JIOTV_EPG")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Load(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("JioTVConfig.Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.name == "Load from valid file" && (tt.c.EPG != true || tt.c.Debug != true || tt.c.Title != "TestTitle") {
				t.Errorf("JioTVConfig.Load() did not load values correctly from file: %+v", tt.c)
			}
			if tt.name == "Load from environment variables" && tt.c.EPG != true {
				t.Errorf("JioTVConfig.Load() did not load EPG from env: %+v", tt.c)
			}
		})
	}
}

func TestJioTVConfig_Load_NormalizesCustomChannelsPathRelativeToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create configs dir: %v", err)
	}

	customChannelsPath := filepath.Join(configDir, "custom-channels.json")
	if err := os.WriteFile(customChannelsPath, []byte(`{"channels":[]}`), 0644); err != nil {
		t.Fatalf("failed to write custom channels file: %v", err)
	}

	configPath := filepath.Join(configDir, "jiotv_go.toml")
	if err := os.WriteFile(configPath, []byte(`custom_channels_file = "custom_channels.json"`), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var cfg JioTVConfig
	if err := cfg.Load(configPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.CustomChannelsFile != customChannelsPath {
		t.Fatalf("expected custom channels path %q, got %q", customChannelsPath, cfg.CustomChannelsFile)
	}
}

func TestJioTVConfig_Load_StripsConfigsPrefixWhenConfigInConfigsDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create configs dir: %v", err)
	}

	customChannelsPath := filepath.Join(configDir, "custom-channels.json")
	if err := os.WriteFile(customChannelsPath, []byte(`{"channels":[]}`), 0644); err != nil {
		t.Fatalf("failed to write custom channels file: %v", err)
	}

	configPath := filepath.Join(configDir, "jiotv_go.toml")
	if err := os.WriteFile(configPath, []byte(`custom_channels_file = "configs/custom-channels.json"`), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var cfg JioTVConfig
	if err := cfg.Load(configPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.CustomChannelsFile != customChannelsPath {
		t.Fatalf("expected custom channels path %q, got %q", customChannelsPath, cfg.CustomChannelsFile)
	}
}

func TestJioTVConfig_Load_EnvOnly_SetsDefaultCustomChannelsFile(t *testing.T) {
	orig := os.Getenv("JIOTV_CUSTOM_CHANNELS_FILE")
	defer func() {
		if orig == "" {
			_ = os.Unsetenv("JIOTV_CUSTOM_CHANNELS_FILE")
		} else {
			_ = os.Setenv("JIOTV_CUSTOM_CHANNELS_FILE", orig)
		}
	}()

	_ = os.Unsetenv("JIOTV_CUSTOM_CHANNELS_FILE")

	var cfg JioTVConfig
	if err := cfg.Load(""); err != nil {
		t.Fatalf("failed to load env-only config: %v", err)
	}

	if strings.TrimSpace(cfg.CustomChannelsFile) == "" {
		t.Fatalf("expected default custom channels file to be set")
	}
}

func TestJioTVConfig_Get(t *testing.T) {
	// Set the global Cfg for Get to work as intended
	Cfg = JioTVConfig{
		EPG:                  true,
		Debug:                false,
		DisableTSHandler:     true,
		DisableLogout:        false,
		DRM:                  true,
		Title:                "TestTitle",
		DisableURLEncryption: false,
		Proxy:                "http://proxy",
		PathPrefix:           "/tmp/jiotv",
		LogPath:              "/tmp/logs",
		LogToStdout:          true,
	}

	tests := []struct {
		name string
		j    *JioTVConfig
		args struct{ key string }
		want interface{}
	}{
		{
			name: "Get EPG",
			j:    &JioTVConfig{},
			args: struct{ key string }{key: "EPG"},
			want: true,
		},
		{
			name: "Get Debug",
			j:    &JioTVConfig{},
			args: struct{ key string }{key: "Debug"},
			want: false,
		},
		{
			name: "Get Title",
			j:    &JioTVConfig{},
			args: struct{ key string }{key: "Title"},
			want: "TestTitle",
		},
		{
			name: "Get Proxy",
			j:    &JioTVConfig{},
			args: struct{ key string }{key: "Proxy"},
			want: "http://proxy",
		},
		{
			name: "Get invalid key",
			j:    &JioTVConfig{},
			args: struct{ key string }{key: "NonExistent"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.j.Get(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JioTVConfig.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommonFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	testFile := filepath.Join(tmpDir, "jiotv_go.yml")
	if err := os.WriteFile(testFile, []byte("epg: true\n"), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	tests := []struct {
		name string
		want string
	}{
		{
			name: "File exists",
			want: "jiotv_go.yml",
		},
		{
			name: "File does not exist",
			want: "",
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if i == 1 {
				_ = os.Remove(testFile)
			}
			got := commonFileExists()
			if got != tt.want {
				t.Errorf("commonFileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

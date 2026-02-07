package config

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// JioTVConfig defines the configuration options for the JioTV client.
// It includes options for enabling features like EPG, debug mode, DRM, etc.
// As well as configuration for credentials, proxies, file paths and more.
type JioTVConfig struct {
	// Enable Or Disable EPG Generation. Default: false
	EPG bool `yaml:"epg" env:"JIOTV_EPG" json:"epg" toml:"epg"`
	// External EPG URL to serve from /epg.xml.gz when local generation is unavailable.
	EPGURL string `yaml:"epg_url" env:"JIOTV_EPG_URL" json:"epg_url" toml:"epg_url"`
	// Enable Or Disable Debug Mode. Default: false
	Debug bool `yaml:"debug" env:"JIOTV_DEBUG" json:"debug" toml:"debug"`
	// Enable Or Disable TS Handler. While TS Handler is enabled, the server will serve the TS files directly from JioTV API. Default: false
	DisableTSHandler bool `yaml:"disable_ts_handler" env:"JIOTV_DISABLE_TS_HANDLER" json:"disable_ts_handler" toml:"disable_ts_handler"`
	// Enable Or Disable Logout feature. Default: true
	DisableLogout bool `yaml:"disable_logout" env:"JIOTV_DISABLE_LOGOUT" json:"disable_logout" toml:"disable_logout"`
	// Enable Or Disable DRM. As DRM is not supported by most of the players, it is disabled by default. Default: false
	DRM bool `yaml:"drm" env:"JIOTV_DRM" json:"drm" toml:"drm"`
	// Title of the webpage. Default: JioTV Go
	Title string `yaml:"title" env:"JIOTV_TITLE" json:"title" toml:"title"`
	// Enable Or Disable URL Encryption. URL Encryption prevents hackers from injecting URLs into the server. Default: true
	DisableURLEncryption bool `yaml:"disable_url_encryption" env:"JIOTV_DISABLE_URL_ENCRYPTION" json:"disable_url_encryption" toml:"disable_url_encryption"`
	// Proxy URL. Proxy is useful to bypass geo-restrictions and ip-restrictions for JioTV API. Default: ""
	Proxy string `yaml:"proxy" env:"JIOTV_PROXY" json:"proxy" toml:"proxy"`
	// PathPrefix is the prefix for all file paths managed by JioTV Go. Default: "$HOME/.jiotv_go"
	PathPrefix string `yaml:"path_prefix" env:"JIOTV_PATH_PREFIX" json:"path_prefix" toml:"path_prefix"`
	// LogPath is the directory for log files. Default: ""
	LogPath string `yaml:"log_path" env:"JIOTV_LOG_PATH" json:"log_path" toml:"log_path"`
	// LogToStdout controls logging to stdout/stderr. Default: true
	LogToStdout bool `yaml:"log_to_stdout" env:"JIOTV_LOG_TO_STDOUT" json:"log_to_stdout" toml:"log_to_stdout"`
	// CustomChannelsURL is an optional remote JSON URL for custom channels.
	CustomChannelsURL string `yaml:"custom_channels_url" env:"JIOTV_CUSTOM_CHANNELS_URL" json:"custom_channels_url" toml:"custom_channels_url"`
	// CustomChannelsFile is the path to custom channels configuration file. Default: ""
	CustomChannelsFile string `yaml:"custom_channels_file" env:"JIOTV_CUSTOM_CHANNELS_FILE" json:"custom_channels_file" toml:"custom_channels_file"`
	// DefaultCategories is the list of category IDs to display on the default web page. Default: []
	DefaultCategories []int `yaml:"default_categories" env:"JIOTV_DEFAULT_CATEGORIES" json:"default_categories" toml:"default_categories"`
	// DefaultLanguages is the list of language IDs to display on the default web page. Default: []
	DefaultLanguages []int `yaml:"default_languages" env:"JIOTV_DEFAULT_LANGUAGES" json:"default_languages" toml:"default_languages"`
}

// Cfg is the global config variable
var Cfg JioTVConfig

// Load loads the JioTVConfig from a file.
// It first checks if a filename is provided, otherwise tries to find a common config file.
// If no file is found, it loads config from environment variables.
// It logs messages about which config source is being used.
func (c *JioTVConfig) Load(filename string) error {
	if filename == "" {
		filename = commonFileExists()
	}
	if filename == "" {
		log.Println("INFO: No config file found, using environment variables")
		if err := cleanenv.ReadEnv(c); err != nil {
			return err
		}
		c.applyDefaults()
		return nil
	}
	log.Println("INFO: Using config file:", filename)
	if err := cleanenv.ReadConfig(filename, c); err != nil {
		return err
	}
	rawCustomChannels := strings.TrimSpace(c.CustomChannelsFile)
	if rawCustomChannels != "" {
		log.Println("INFO: Custom channels file (raw):", rawCustomChannels)
	}
	c.normalizePaths(filename)
	resolvedCustomChannels := strings.TrimSpace(c.CustomChannelsFile)
	if resolvedCustomChannels != "" {
		log.Println("INFO: Custom channels file (resolved):", resolvedCustomChannels)
		log.Println("INFO: Custom channels file exists:", fileExists(resolvedCustomChannels))
	}
	c.applyDefaults()
	return nil
}

func (c *JioTVConfig) applyDefaults() {
	if strings.TrimSpace(c.CustomChannelsFile) == "" {
		c.CustomChannelsFile = filepath.Join("configs", "custom-channels.json")
	}
	if strings.TrimSpace(c.EPGURL) == "" {
		c.EPGURL = "https://avkb.short.gy/jioepg.xml.gz"
	}
	if strings.TrimSpace(c.CustomChannelsURL) == "" {
		c.CustomChannelsURL = "https://raw.githubusercontent.com/atanuroy22/iptv/refs/heads/main/output/custom-channels.json"
	}
}

func (c *JioTVConfig) normalizePaths(configFilePath string) {
	raw := strings.TrimSpace(c.CustomChannelsFile)
	if raw == "" {
		return
	}
	if filepath.IsAbs(raw) {
		return
	}
	if fileExists(raw) {
		c.CustomChannelsFile = raw
		return
	}

	configDir := filepath.Dir(configFilePath)
	var relCandidates []string
	relCandidates = append(relCandidates, raw)

	rawSlash := filepath.ToSlash(raw)
	if strings.HasPrefix(rawSlash, "configs/") && filepath.Base(configDir) == "configs" {
		relCandidates = append(relCandidates, strings.TrimPrefix(rawSlash, "configs/"))
	}

	base := filepath.Base(raw)
	altBase := strings.ReplaceAll(base, "custom_channels", "custom-channels")
	if altBase != base {
		relCandidates = append(relCandidates, altBase)
		if strings.HasPrefix(rawSlash, "configs/") && filepath.Base(configDir) == "configs" {
			relCandidates = append(relCandidates, strings.TrimPrefix(filepath.ToSlash(altBase), "configs/"))
		}
	}

	for _, rel := range relCandidates {
		rel = filepath.Clean(filepath.FromSlash(rel))
		if rel == "" || rel == "." {
			continue
		}
		candidate := filepath.Join(configDir, rel)
		if fileExists(candidate) {
			c.CustomChannelsFile = candidate
			return
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Get retrieves the value of the config field specified by key.
// It uses reflection to get the field value from the global Cfg variable.
// Returns the field value as an interface{}, or nil if the field is invalid.
func (*JioTVConfig) Get(key string) interface{} {
	r := reflect.ValueOf(Cfg)
	f := reflect.Indirect(r).FieldByName(key)
	if f.IsValid() {
		return f.Interface()
	}
	return nil
}

// commonFileExists checks for the existence of common config
// file names and returns the first one found. It searches
// for config files in the following formats:
//   - jiotv_go.{yml,toml,json,yaml}
//   - config.{json,yml,toml,yaml}
//
// If no file is found, an empty string is returned.
func commonFileExists() string {
	commonFiles := []string{"jiotv_go.yml", "jiotv_go.yaml", "jiotv_go.toml", "jiotv_go.json", "config.json", "config.yml", "config.toml", "config.yaml"}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	for _, filename := range commonFiles {
		// check above common files in current directory
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
		// check in configs directory
		if _, err := os.Stat("configs/" + filename); err == nil {
			return "configs/" + filename
		}
		// check in executable directory
		exeFile := filepath.Join(exeDir, filename)
		if _, err := os.Stat(exeFile); err == nil {
			return exeFile
		}
		// check in executable directory configs
		exeConfigFile := filepath.Join(exeDir, "configs", filename)
		if _, err := os.Stat(exeConfigFile); err == nil {
			return exeConfigFile
		}
	}
	return ""
}

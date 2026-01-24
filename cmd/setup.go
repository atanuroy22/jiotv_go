package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jiotv-go/jiotv_go/v3/pkg/television"
)

const (
	RepoOwner       = "atanuroy22"
	RepoName        = "jiotv_go"
	Branch          = "develop"
	BaseURL         = "https://raw.githubusercontent.com/" + RepoOwner + "/" + RepoName + "/" + Branch
	JioTVGoTomlURL  = BaseURL + "/configs/jiotv_go.toml"
	CustomChJSONURL = BaseURL + "/configs/custom-channels.json"

	AllM3UURL = "https://atanuroy22.github.io/iptv/output/all.m3u"

	ConfigDir = "configs"
)

// SetupEnvironment performs the startup setup:
// 1. Downloads config files (overwriting existing ones).
// 2. Fetches M3U playlists.
// 3. Adds channels from M3U to custom-channels.json.
func SetupEnvironment() error {
	fmt.Println("INFO: Starting environment setup...")

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	configDir := filepath.Join(exeDir, ConfigDir)

	// Ensure configs directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create configs directory: %w", err)
	}

	// 1. Download jiotv_go.toml
	fmt.Println("INFO: Downloading jiotv_go.toml...")
	tomlPath := filepath.Join(configDir, "jiotv_go.toml")
	if err := downloadFile(JioTVGoTomlURL, tomlPath); err != nil {
		return fmt.Errorf("failed to download jiotv_go.toml: %w", err)
	}

	// Patch jiotv_go.toml to point to configs/custom-channels.json
	// The repo has a mismatch (underscore vs hyphen) and we also want to ensure it looks in configs/
	if content, err := os.ReadFile(tomlPath); err == nil {
		newContent := strings.Replace(string(content), `custom_channels_file = "custom_channels.json"`, `custom_channels_file = "configs/custom-channels.json"`, 1)
		if err := os.WriteFile(tomlPath, []byte(newContent), 0644); err != nil {
			fmt.Printf("WARN: Failed to patch jiotv_go.toml: %v\n", err)
		}
	}

	// 2. Download custom-channels.json
	fmt.Println("INFO: Downloading custom-channels.json...")
	customChPath := filepath.Join(configDir, "custom-channels.json")
	if err := downloadFile(CustomChJSONURL, customChPath); err != nil {
		return fmt.Errorf("failed to download custom-channels.json: %w", err)
	}

	// 3. Load the downloaded custom-channels.json
	var customChannels television.CustomChannelsConfig
	data, err := os.ReadFile(customChPath)
	if err != nil {
		return fmt.Errorf("failed to read custom-channels.json: %w", err)
	}
	if err := json.Unmarshal(data, &customChannels); err != nil {
		return fmt.Errorf("failed to parse custom-channels.json: %w", err)
	}

	fmt.Printf("INFO: Fetching channels from %s...\n", AllM3UURL)
	newChannels, err := fetchAndParseM3U(AllM3UURL)
	if err != nil {
		return fmt.Errorf("failed to fetch/parse M3U from %s: %w", AllM3UURL, err)
	}

	existingIDs := make(map[string]struct{}, len(customChannels.Channels))
	for _, ch := range customChannels.Channels {
		existingIDs[ch.ID] = struct{}{}
	}

	var addedCount int
	for _, ch := range newChannels {
		if _, ok := existingIDs[ch.ID]; ok {
			continue
		}
		customChannels.Channels = append(customChannels.Channels, ch)
		existingIDs[ch.ID] = struct{}{}
		addedCount++
	}

	updatedData, err := json.MarshalIndent(customChannels, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated custom channels: %w", err)
	}

	tmpCustomChPath := customChPath + ".tmp"
	if err := os.WriteFile(tmpCustomChPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated custom-channels.json: %w", err)
	}

	if err := os.Rename(tmpCustomChPath, customChPath); err != nil {
		_ = os.Remove(tmpCustomChPath)
		return fmt.Errorf("failed to replace custom-channels.json: %w", err)
	}

	fmt.Printf("INFO: Environment setup complete. Added %d channels.\n", addedCount)
	return nil
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchAndParseM3U(url string) ([]television.CustomChannel, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var channels []television.CustomChannel
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var currentChannel television.CustomChannel
	isInfoLine := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			isInfoLine = true
			currentChannel = television.CustomChannel{}
			// Parse metadata
			// Example: #EXTINF:-1 tvg-id="Sony_HD" tvg-logo="http://..." group-title="Entertainment",Sony HD

			// Extract Name (after last comma)
			lastCommaIdx := strings.LastIndex(line, ",")
			if lastCommaIdx != -1 {
				currentChannel.Name = strings.TrimSpace(line[lastCommaIdx+1:])
			}

			// Extract Logo
			currentChannel.LogoURL = extractAttribute(line, "tvg-logo")

			// Extract ID
			id := extractAttribute(line, "tvg-id")
			if id == "" {
				// Generate a random ID or use Name
				id = strings.ReplaceAll(strings.ToLower(currentChannel.Name), " ", "_")
			}
			currentChannel.ID = id

			// Map Category (simple mapping or default)
			// group-title="Entertainment"
			groupTitle := extractAttribute(line, "group-title")
			currentChannel.Category = mapCategory(groupTitle)

			// Set defaults
			currentChannel.Language = mapLanguage(extractAttribute(line, "tvg-language"))
			currentChannel.IsHD = strings.Contains(strings.ToUpper(currentChannel.Name), "HD")

		} else if strings.HasPrefix(line, "#") && isInfoLine {
			continue
		} else if !strings.HasPrefix(line, "#") && isInfoLine {
			// This is the URL line
			currentChannel.URL = line
			if strings.HasPrefix(strings.ToLower(currentChannel.URL), "https://") {
				channels = append(channels, currentChannel)
			}
			isInfoLine = false
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return channels, nil
}

func extractAttribute(line, key string) string {
	keyStr := key + "=\""
	start := strings.Index(line, keyStr)
	if start == -1 {
		return ""
	}
	start += len(keyStr)
	end := strings.Index(line[start:], "\"")
	if end == -1 {
		return ""
	}
	return line[start : start+end]
}

func mapCategory(group string) int {
	// Simple mapping based on known categories in pkg/television/types.go
	// 5: "Entertainment", 6: "Movies", 7: "Kids", 8: "Sports",
	group = strings.ToLower(group)
	if strings.Contains(group, "entertainment") {
		return 5
	}
	if strings.Contains(group, "movie") {
		return 6
	}
	if strings.Contains(group, "kid") {
		return 7
	}
	if strings.Contains(group, "sport") {
		return 8
	}
	if strings.Contains(group, "news") {
		return 12 // Assuming 12 is News, check types.go later if needed, but 12 is common
	}
	// Default
	return 0 // All Categories
}

func GetConfigDir() string {
	return ConfigDir
}

func mapLanguage(lang string) int {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "hindi":
		return 1
	case "marathi":
		return 2
	case "punjabi":
		return 3
	case "urdu":
		return 4
	case "bengali":
		return 5
	case "english":
		return 6
	case "malayalam":
		return 7
	case "tamil":
		return 8
	case "gujarati":
		return 9
	case "odia", "oriya":
		return 10
	case "telugu":
		return 11
	case "bhojpuri":
		return 12
	case "kannada":
		return 13
	case "assamese":
		return 14
	case "nepali":
		return 15
	case "french":
		return 16
	case "":
		return 0
	default:
		return 18
	}
}

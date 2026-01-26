package cmd

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	baseDir := chooseConfigBaseDir(exeDir)
	configDir := filepath.Join(baseDir, ConfigDir)
	fmt.Printf("INFO: Executable dir: %s\n", exeDir)
	fmt.Printf("INFO: Config base dir: %s\n", baseDir)
	fmt.Printf("INFO: Config dir: %s\n", configDir)

	// Ensure configs directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create configs directory: %w", err)
	}

	// 1. Download jiotv_go.toml
	fmt.Println("INFO: Downloading jiotv_go.toml...")
	tomlPath := filepath.Join(configDir, "jiotv_go.toml")
	fmt.Printf("INFO: Config TOML path: %s\n", tomlPath)
	if err := downloadFile(JioTVGoTomlURL, tomlPath); err != nil {
		if !pathExists(tomlPath) {
			altToml := filepath.Join("configs", "jiotv_go.toml")
			if pathExists(altToml) {
				fmt.Printf("WARN: Failed to download jiotv_go.toml, using existing: %s\n", altToml)
				tomlPath = altToml
				configDir = filepath.Dir(altToml)
			} else {
				return fmt.Errorf("failed to download jiotv_go.toml: %w", err)
			}
		} else {
			fmt.Printf("WARN: Failed to download jiotv_go.toml, using existing: %s\n", tomlPath)
		}
	}

	if err := ensureCustomChannelsSettingInToml(tomlPath); err != nil {
		fmt.Printf("WARN: Failed to patch jiotv_go.toml: %v\n", err)
	} else {
		if v, err := readTomlCustomChannelsValue(tomlPath); err == nil && strings.TrimSpace(v) != "" {
			fmt.Printf("INFO: TOML custom_channels_file: %s\n", v)
		}
	}

	// 2. Download custom-channels.json
	fmt.Println("INFO: Downloading custom-channels.json...")
	customChPath := filepath.Join(configDir, "custom-channels.json")
	fmt.Printf("INFO: Custom channels JSON path: %s\n", customChPath)
	fmt.Printf("INFO: Custom channels alt JSON path: %s\n", filepath.Join(configDir, "custom_channels.json"))
	if err := downloadFile(CustomChJSONURL, customChPath); err != nil {
		if !pathExists(customChPath) {
			altCustomCh := filepath.Join("configs", "custom-channels.json")
			if pathExists(altCustomCh) {
				fmt.Printf("WARN: Failed to download custom-channels.json, using existing: %s\n", altCustomCh)
				customChPath = altCustomCh
				configDir = filepath.Dir(altCustomCh)
			} else {
				return fmt.Errorf("failed to download custom-channels.json: %w", err)
			}
		} else {
			fmt.Printf("WARN: Failed to download custom-channels.json, using existing: %s\n", customChPath)
		}
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
		fmt.Printf("WARN: Failed to fetch/parse M3U from %s: %v\n", AllM3UURL, err)
		fmt.Println("INFO: Environment setup complete. Added 0 channels.")
		return nil
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

func chooseConfigBaseDir(exeDir string) string {
	if runtime.GOOS == "windows" {
		return exeDir
	}

	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return exeDir
	}

	if strings.HasPrefix(exeDir, "/data/data/com.termux/files/usr/") {
		return cwd
	}

	if !dirWritable(exeDir) && dirWritable(cwd) {
		return cwd
	}

	return exeDir
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".jiotv_go_tmp_*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func ensureCustomChannelsSettingInToml(tomlPath string) error {
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return err
	}

	desired := `custom_channels_file = "configs/custom-channels.json"`

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var out strings.Builder
	out.Grow(len(data) + 64)

	found := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "#") {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(trimmed, "custom_channels_file") {
			out.WriteString(desired)
			out.WriteByte('\n')
			found = true
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if !found {
		out.WriteString(desired)
		out.WriteByte('\n')
	}

	return os.WriteFile(tomlPath, []byte(out.String()), 0644)
}

func readTomlCustomChannelsValue(tomlPath string) (string, error) {
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "custom_channels_file") {
			return trimmed, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var setupHTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: setupHTTPTransport(),
}

func setupHTTPTransport() http.RoundTripper {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}

	transport := defaultTransport.Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs: setupRootCAs(),
	}
	return transport
}

func setupRootCAs() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if pool == nil || err != nil {
		pool = x509.NewCertPool()
	}

	var certFiles []string
	if v := strings.TrimSpace(os.Getenv("SSL_CERT_FILE")); v != "" {
		certFiles = append(certFiles, v)
	}
	if v := strings.TrimSpace(os.Getenv("SSL_CERT_DIR")); v != "" {
		certFiles = append(certFiles, v)
	}

	certFiles = append(certFiles,
		"/data/data/com.termux/files/usr/etc/tls/cert.pem",
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
	)

	for _, p := range certFiles {
		info, statErr := os.Stat(p)
		if statErr != nil || info.IsDir() {
			continue
		}
		if pem, readErr := os.ReadFile(p); readErr == nil {
			_ = pool.AppendCertsFromPEM(pem)
		}
	}

	return pool
}

func downloadFile(urlStr, filePath string) error {
	var lastErr error
	for _, candidate := range fallbackURLs(urlStr) {
		if err := downloadFileOnce(candidate, filePath); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate URLs")
	}
	return lastErr
}

func downloadFileOnce(urlStr, filePath string) error {
	resp, err := httpGetOK(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmpPath := filePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func fetchAndParseM3U(urlStr string) ([]television.CustomChannel, error) {
	var lastErr error
	for _, candidate := range fallbackURLs(urlStr) {
		resp, err := httpGetOK(candidate)
		if err != nil {
			lastErr = err
			continue
		}

		channels, parseErr := parseM3U(resp.Body)
		_ = resp.Body.Close()
		if parseErr != nil {
			lastErr = parseErr
			continue
		}

		return channels, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate URLs")
	}
	return nil, lastErr
}

func httpGetOK(urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "jiotv_go")

	resp, err := setupHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	return resp, nil
}

func fallbackURLs(urlStr string) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}

	add(urlStr)
	add(jsDelivrFallback(urlStr))
	return out
}

func jsDelivrFallback(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	switch parsed.Host {
	case "raw.githubusercontent.com":
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) < 4 {
			return ""
		}
		owner, repo, ref := parts[0], parts[1], parts[2]
		rest := strings.Join(parts[3:], "/")
		if owner == "" || repo == "" || ref == "" || rest == "" {
			return ""
		}
		return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", owner, repo, ref, rest)
	default:
		if !strings.HasSuffix(parsed.Host, ".github.io") {
			return ""
		}
		if strings.HasPrefix(parsed.Host, "cdn.jsdelivr.net") || strings.HasSuffix(parsed.Host, "jsdelivr.net") {
			return ""
		}
		owner := strings.TrimSuffix(parsed.Host, ".github.io")
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) < 2 {
			return ""
		}
		repo := parts[0]
		rest := strings.Join(parts[1:], "/")
		if owner == "" || repo == "" || rest == "" {
			return ""
		}
		return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@gh-pages/%s", owner, repo, rest)
	}
}

func parseM3U(r io.Reader) ([]television.CustomChannel, error) {

	var channels []television.CustomChannel
	scanner := bufio.NewScanner(r)
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

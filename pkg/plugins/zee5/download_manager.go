package zee5

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jiotv-go/jiotv_go/v3/internal/config"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

var (
	downloadHTTPClient = &http.Client{
		Timeout: 30 * time.Second,
	}
)

const defaultZee5DataURL = "https://raw.githubusercontent.com/atanuroy22/zee5/refs/heads/main/data.json"

// DownloadZee5Data downloads zee5 data from the configured URL and saves it to the file path
func DownloadZee5Data() error {
	dataURL := strings.TrimSpace(config.Cfg.Zee5DataURL)
	if dataURL == "" {
		dataURL = defaultZee5DataURL
	}

	dataFilePath := strings.TrimSpace(config.Cfg.Zee5DataFile)
	if dataFilePath == "" {
		return fmt.Errorf("zee5_data_file not configured")
	}

	// Download from the primary URL
	data, err := downloadZee5DataFromURL(dataURL)
	if err != nil {
		utils.SafeLogf("WARN: Failed to download zee5 data from primary URL: %v", err)

		// Try fallback URLs
		fallbackURLs := []string{
			// jsDelivr CDN fallback
			"https://cdn.jsdelivr.net/gh/atanuroy22/zee5@main/data.json",
			// ghproxy fallback for Chinese users
			"https://ghproxy.com/https://raw.githubusercontent.com/atanuroy22/zee5/refs/heads/main/data.json",
		}

		for _, fallbackURL := range fallbackURLs {
			utils.SafeLogf("INFO: Trying fallback URL: %s", fallbackURL)
			if fallbackData, fallbackErr := downloadZee5DataFromURL(fallbackURL); fallbackErr == nil {
				data = fallbackData
				err = nil
				break
			}
		}

		// If all downloads failed, check if we have a local copy to keep
		if err != nil {
			if fileExistsZee5(dataFilePath) {
				utils.SafeLogf("WARN: Failed to download zee5 data (keeping existing file): %v", err)
				return nil
			}
			utils.SafeLogf("WARN: Failed to download zee5 data and no local file exists: %v", err)
			return err
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(dataFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		utils.SafeLogf("ERROR: Failed to create directory for zee5 data: %v", err)
		return err
	}

	// Save the data to file
	if err := SaveZee5Data(dataFilePath, data); err != nil {
		utils.SafeLogf("ERROR: Failed to save zee5 data: %v", err)
		return err
	}

	// Update the cached data
	zee5DataMu.Lock()
	zee5DataCache = data
	zee5DataMu.Unlock()

	utils.SafeLogf("INFO: Successfully downloaded and cached %d Zee5 channels", len(data.Data))
	return nil
}

// stripBOM removes a UTF-8 BOM from the beginning of data if present
func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

// downloadZee5DataFromURL downloads zee5 data from the specified URL
func downloadZee5DataFromURL(url string) (*DataFile, error) {
	resp, err := downloadHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zee5 data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to download zee5 data: HTTP %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Strip UTF-8 BOM if present
	body = stripBOM(body)

	var data DataFile
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse zee5 data: %w", err)
	}

	return &data, nil
}

// RefreshZee5DataFromURL downloads zee5 data and updates the cached version
func RefreshZee5DataFromURL() error {
	return DownloadZee5Data()
}

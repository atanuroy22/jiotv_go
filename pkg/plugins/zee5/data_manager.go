package zee5

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jiotv-go/jiotv_go/v3/internal/config"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

var (
	// zee5DataCache holds the cached zee5 data
	zee5DataCache *DataFile
	zee5DataMu    sync.RWMutex
)

// InitZee5Data initializes zee5 data at startup if configured
func InitZee5Data() {
	loadAndCacheZee5Data()
}

// ReloadZee5Data reloads zee5 data from file
func ReloadZee5Data() {
	if config.Cfg.Zee5DataFile != "" {
		loadAndCacheZee5Data()
	}
}

// loadAndCacheZee5Data loads zee5 data from file or embedded source and caches it
func loadAndCacheZee5Data() {
	data, err := LoadZee5Data(config.Cfg.Zee5DataFile)
	if err != nil {
		utils.SafeLogf("WARN: Error loading zee5 data: %v", err)
		data = nil
	}

	zee5DataMu.Lock()
	zee5DataCache = data
	zee5DataMu.Unlock()

	if data != nil && len(data.Data) > 0 {
		utils.SafeLogf("INFO: Zee5 cached %d channels", len(data.Data))
	} else {
		utils.SafeLogf("WARN: Zee5 data empty or failed to load, channels will not appear")
	}
}

// GetCachedZee5Data returns the cached zee5 data
func GetCachedZee5Data() *DataFile {
	zee5DataMu.RLock()
	defer zee5DataMu.RUnlock()
	return zee5DataCache
}

// LoadZee5Data loads zee5 data from the configured file path only.
// Returns an error if the file does not exist or cannot be parsed.
func LoadZee5Data(filePath string) (*DataFile, error) {
	if filePath == "" || !fileExistsZee5(filePath) {
		return nil, fmt.Errorf("zee5 data file not found: %s", filePath)
	}
	return loadZee5DataFromFile(filePath)
}

// loadZee5DataFromFile loads zee5 data from the specified file path
func loadZee5DataFromFile(filePath string) (*DataFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var df DataFile
	if err := json.Unmarshal(data, &df); err != nil {
		return nil, err
	}

	return &df, nil
}

// fileExistsZee5 checks if a file exists
func fileExistsZee5(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SaveZee5Data saves zee5 data to file
func SaveZee5Data(filePath string, data *DataFile) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

package television

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v3"

	"github.com/jiotv-go/jiotv_go/v3/internal/config"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/headers"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/urls"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

const (
	// JioTV API domain constants
	JIOTV_API_DOMAIN = urls.JioTVAPIDomain
	TV_MEDIA_DOMAIN  = urls.TVMediaDomain
	JIOTV_CDN_DOMAIN = urls.JioTVCDNDomain

	// URL for fetching channels from JioTV API
	CHANNELS_API_URL = urls.ChannelsAPIURL
	// Error message for unsupported custom channels file formats
	errUnsupportedChannelsFormat = constants.ErrUnsupportedChannelsFormat
	// Maximum recommended number of custom channels before performance warnings
	maxRecommendedChannels = constants.MaxRecommendedChannels
)

// logExcessiveChannelsWarning logs a comprehensive warning when the number of custom channels exceeds the recommended limit
func logExcessiveChannelsWarning(channelCount int, context string) {
	if channelCount <= maxRecommendedChannels {
		return
	}

	utils.SafeLogf("WARNING: %s %d custom channels, which exceeds the recommended limit of %d channels.", context, channelCount, maxRecommendedChannels)
	utils.SafeLog("WARNING: Large numbers of custom channels may impact performance:")
	utils.SafeLog("  - Slower channel listing and filtering operations")
	utils.SafeLog("  - Increased memory usage")
	utils.SafeLog("  - Longer startup times")
	utils.SafeLog("  - Potential UI responsiveness issues")
	utils.SafeLog("Consider splitting channels into multiple configuration files or reducing the total number.")
}

var (
	// customChannelsCacheMap holds cached custom channels indexed by ID for efficient lookups
	customChannelsCacheMap map[string]Channel
	customChannelsMu       sync.RWMutex
)

// New function creates a new Television instance with the provided credentials
func New(credentials *utils.JIOTV_CREDENTIALS) *Television {
	// Check if credentials are provided
	if credentials == nil {
		// If credentials are not provided, set them to empty strings
		credentials = &utils.JIOTV_CREDENTIALS{
			AccessToken: "",
			SSOToken:    "",
			CRM:         "",
			UniqueID:    "",
		}
	}
	headers := map[string]string{
		"Content-type":    "application/x-www-form-urlencoded",
		"appkey":          "NzNiMDhlYzQyNjJm",
		"channel_id":      "",
		"crmid":           credentials.CRM,
		"userId":          credentials.CRM,
		"deviceId":        utils.GetDeviceID(),
		"devicetype":      "phone",
		"isott":           "false",
		"languageId":      "6",
		"lbcookie":        "1",
		"os":              "android",
		"osVersion":       "13",
		"subscriberId":    credentials.CRM,
		"uniqueId":        credentials.UniqueID,
		headers.UserAgent: headers.UserAgentOkHttp,
		"usergroup":       "tvYR7NSNn7rymo3F",
		"versionCode":     headers.VersionCode389,
	}

	// Create a fasthttp.Client
	client := utils.GetRequestClient()

	// Return a new Television instance
	return &Television{
		AccessToken: credentials.AccessToken,
		SsoToken:    credentials.SSOToken,
		Crm:         credentials.CRM,
		UniqueID:    credentials.UniqueID,
		Headers:     headers,
		Client:      client,
	}
}

// InitCustomChannels initializes custom channels at startup if configured
func InitCustomChannels() {
	if config.Cfg.CustomChannelsFile != "" {
		loadAndCacheCustomChannels()
	}
}

func ReloadCustomChannels() {
	if config.Cfg.CustomChannelsFile != "" {
		loadAndCacheCustomChannels()
	}
}

// getCustomChannelByID efficiently looks up a custom channel by ID
func getCustomChannelByID(channelID string) (Channel, bool) {
	customChannelsMu.RLock()
	defer customChannelsMu.RUnlock()

	if customChannelsCacheMap == nil {
		return Channel{}, false
	}

	channel, exists := customChannelsCacheMap[channelID]
	return channel, exists
}

// GetCustomChannelByID efficiently looks up a custom channel by ID (public version)
func GetCustomChannelByID(channelID string) (Channel, bool) {
	return getCustomChannelByID(channelID)
}

// loadAndCacheCustomChannels loads custom channels from file and caches them
func loadAndCacheCustomChannels() {
	channels, err := LoadCustomChannels(config.Cfg.CustomChannelsFile)
	next := make(map[string]Channel)
	if err != nil {
		utils.SafeLogf("Error loading custom channels: %v", err)
	} else {
		for _, channel := range channels {
			next[channel.ID] = channel
		}

		logExcessiveChannelsWarning(len(channels), "Cached")
	}

	customChannelsMu.Lock()
	customChannelsCacheMap = next
	customChannelsMu.Unlock()
}

// Live method generates m3u8 link from JioTV API with the provided channel ID
func (tv *Television) Live(channelID string) (*LiveURLOutput, error) {
	// If channelID starts with sl, then it is a Sony Channel
	if len(channelID) >= 2 && channelID[:2] == "sl" {
		return getSLChannel(channelID)
	}

	formData := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(formData)

	formData.Add("channel_id", channelID)
	formData.Add("stream_type", "Seek")
	formData.Add("begin", utils.GenerateCurrentTime())
	formData.Add("srno", utils.GenerateDate())

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	// Copy headers from the Television headers map to the request
	for key, value := range tv.Headers {
		req.Header.Set(key, value)
	}

	// Always use the v1.1 API endpoint
	url := "https://" + JIOTV_API_DOMAIN + urls.PlaybackAPIPath
	req.Header.Set(headers.AccessToken, tv.AccessToken)
	req.SetRequestURI(url)
	req.Header.SetMethod("POST")

	// Encode the form data and set it as the request body
	req.SetBody(formData.QueryString())

	req.Header.Set("channel_id", channelID)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	// Perform the HTTP POST request
	if err := tv.Client.Do(req, resp); err != nil {
		if strings.Contains(err.Error(), "server closed connection before returning the first response byte") {
			utils.Log.Println("Retrying the request...")
			return tv.Live(channelID)
		}
		utils.Log.Panic(err)
		return nil, err
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		// Store the response body as a string
		response := string(resp.Body())

		// Log headers and request data
		utils.Log.Println("Request headers:", req.Header.String())
		utils.Log.Println("Request data:", formData.String())
		utils.Log.Panicln("Response: ", response)

		return nil, fmt.Errorf("Request failed with status code: %d\nresponse: %s", resp.StatusCode(), response)
	}

	var result LiveURLOutput
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		utils.Log.Panic(err)
		return nil, err
	}

	// Extract hdnea from any URL fields in the response (Live does not set Set-Cookie)
	extractHdneaFromURL := func(u string) string {
		if u == "" {
			return ""
		}
		idx := strings.Index(u, "hdnea=")
		if idx == -1 {
			return ""
		}
		// token starts after hdnea=
		token := u[idx+len("hdnea="):]
		if i := strings.IndexByte(token, '&'); i != -1 {
			token = token[:i]
		}
		return token
	}
	hdnea := extractHdneaFromURL(result.Bitrates.Auto)
	if hdnea == "" {
		hdnea = extractHdneaFromURL(result.Mpd.Result)
	}
	result.Hdnea = hdnea

	// If hdnea exists and URLs don't already have it, append as query param
	if hdnea != "" {
		appendHdnea := func(u string) string {
			if u == "" {
				return u
			}
			if strings.Contains(u, "hdnea=") {
				return u
			}
			sep := "?"
			if strings.Contains(u, "?") {
				sep = "&"
			}
			return u + sep + "hdnea=" + hdnea
		}
		result.Bitrates.Auto = appendHdnea(result.Bitrates.Auto)
		result.Bitrates.High = appendHdnea(result.Bitrates.High)
		result.Bitrates.Medium = appendHdnea(result.Bitrates.Medium)
		result.Bitrates.Low = appendHdnea(result.Bitrates.Low)
		result.Result = appendHdnea(result.Result)
		if result.Mpd.Result != "" {
			result.Mpd.Result = appendHdnea(result.Mpd.Result)
		}
		if result.Mpd.Key != "" {
			result.Mpd.Key = appendHdnea(result.Mpd.Key)
		}
	}

	return &result, nil
}

// Render method does HTTP GET request to the provided URL and return the response body
func (tv *Television) Render(url string) ([]byte, int, string) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(url)
	req.Header.SetMethod("GET")

	// Copy headers from the Television headers map to the request
	for key, value := range tv.Headers {
		req.Header.Set(key, value)
	}

	// If hdnea is provided as query param on URL, also send it as cookie __hdnea__ per downstream requirement
	if strings.Contains(url, "hdnea=") {
		// quick parse to extract value
		q := url[strings.Index(url, "?")+1:]
		for _, p := range strings.Split(q, "&") {
			if strings.HasPrefix(p, "hdnea=") {
				token := strings.TrimPrefix(p, "hdnea=")
				req.Header.SetCookie("__hdnea__", token)
				break
			}
		}
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	// Perform the HTTP GET request
	if err := tv.Client.Do(req, resp); err != nil {
		utils.Log.Panic(err)
	}

	buf := resp.Body()
	// Capture any __hdnea__ Set-Cookie returned by upstream so caller can set cookie on client
	var newHdnea string
	// Iterate Set-Cookie headers (avoid deprecated VisitAll)
	for _, v := range resp.Header.PeekAll("Set-Cookie") {
		sv := string(v)
		if strings.HasPrefix(sv, "__hdnea__=") {
			token := sv[len("__hdnea__="):]
			if i := strings.IndexByte(token, ';'); i != -1 {
				token = token[:i]
			}
			newHdnea = token
		}
	}

	return buf, resp.StatusCode(), newHdnea
}

// detectAndParseFormat attempts to detect the format of custom channels data and parse it
func detectAndParseFormat(data []byte, filePath string) (CustomChannelsConfig, error) {
	var customConfig CustomChannelsConfig

	// Determine file format by extension and parse accordingly, fallback to content-based detection
	if strings.HasSuffix(filePath, ".json") {
		err := json.Unmarshal(data, &customConfig)
		return customConfig, err
	}

	if strings.HasSuffix(filePath, ".yml") || strings.HasSuffix(filePath, ".yaml") {
		err := yaml.Unmarshal(data, &customConfig)
		return customConfig, err
	}

	// Fallback: try to detect format by content for unknown extensions
	trimmed := strings.TrimSpace(string(data))

	// For unsupported extensions, require non-empty content
	if trimmed == "" {
		return customConfig, errors.New(errUnsupportedChannelsFormat)
	}

	// Try JSON if content starts with '{' or '['
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		err := json.Unmarshal(data, &customConfig)
		if err == nil {
			return customConfig, nil
		}
		// If JSON parsing failed, try YAML as fallback
		err = yaml.Unmarshal(data, &customConfig)
		if err != nil {
			return customConfig, errors.New(errUnsupportedChannelsFormat)
		}
		return customConfig, nil
	}

	// Try YAML for other content
	err := yaml.Unmarshal(data, &customConfig)
	if err != nil {
		return customConfig, errors.New(errUnsupportedChannelsFormat)
	}
	return customConfig, nil
}

// LoadCustomChannels loads custom channels from configuration file
func LoadCustomChannels(filePath string) ([]Channel, error) {
	if filePath == "" {
		return []Channel{}, nil
	}

	// Check if file exists and read it
	fileResult := utils.CheckAndReadFile(filePath)
	if !fileResult.Exists {
		utils.SafeLogf("Custom channels file not found: %s", filePath)
		if isDefaultCustomChannelsPath(filePath) {
			customConfig, err := loadBuiltInCustomChannelsConfig()
			if err == nil {
				return convertCustomConfigToChannels(customConfig), nil
			}
		}
		return []Channel{}, nil
	}

	if fileResult.Error != nil {
		return nil, fileResult.Error
	}

	// Parse the file using format detection
	customConfig, err := detectAndParseFormat(fileResult.Data, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse custom channels file: %w", err)
	}

	channels := convertCustomConfigToChannels(customConfig)

	utils.SafeLogf("Loaded %d custom channels from %s", len(channels), filePath)

	// Warn user about performance implications if too many channels
	logExcessiveChannelsWarning(len(channels), "You have loaded")
	return channels, nil
}

func getCustomChannels() []Channel {
	customChannelsMu.RLock()
	defer customChannelsMu.RUnlock()

	var customChannels []Channel
	for _, channel := range customChannelsCacheMap {
		customChannels = append(customChannels, channel)
	}
	return customChannels
}

func isDefaultCustomChannelsPath(filePath string) bool {
	base := strings.ToLower(filepath.Base(filePath))
	if base != "custom-channels.json" && base != "custom_channels.json" && base != "custom-channels.yml" && base != "custom_channels.yml" && base != "custom-channels.yaml" && base != "custom_channels.yaml" {
		return false
	}
	return true
}

func loadBuiltInCustomChannelsConfig() (CustomChannelsConfig, error) {
	var customConfig CustomChannelsConfig
	if err := json.Unmarshal([]byte(builtInCustomChannelsJSON), &customConfig); err != nil {
		return CustomChannelsConfig{}, err
	}
	return customConfig, nil
}

func convertCustomConfigToChannels(customConfig CustomChannelsConfig) []Channel {
	var channels []Channel
	for _, customChannel := range customConfig.Channels {
		channelID := customChannel.ID
		if !strings.HasPrefix(channelID, "cc_") {
			channelID = "cc_" + channelID
		}

		channel := Channel{
			ID:       channelID,
			Name:     customChannel.Name,
			URL:      customChannel.URL,
			LogoURL:  customChannel.LogoURL,
			Category: customChannel.Category,
			Language: customChannel.Language,
			IsHD:     customChannel.IsHD,
		}
		channels = append(channels, channel)
	}
	return channels
}

const builtInCustomChannelsJSON = `{
  "channels": [
    {
      "id": "custom_news_1",
      "name": "Sample News Channel",
      "url": "https://example.com/news/playlist.m3u8",
      "logo_url": "https://example.com/logos/news.png",
      "category": 12,
      "language": 6,
      "is_hd": true
    },
    {
      "id": "custom_entertainment_1",
      "name": "Sample Entertainment Channel",
      "url": "https://example.com/entertainment/playlist.m3u8",
      "logo_url": "https://example.com/logos/entertainment.png",
      "category": 5,
      "language": 1,
      "is_hd": false
    }
  ]
}`

// Channels fetch channels from JioTV API and merge with custom channels
func Channels() (ChannelsResponse, error) {
	// Create a fasthttp.Client
	client := utils.GetRequestClient()

	// Set up request headers
	requestHeaders := map[string]string{
		headers.UserAgent:  headers.UserAgentOkHttp,
		headers.Accept:     headers.AcceptJSON,
		headers.DeviceType: headers.DeviceTypePhone,
		headers.OS:         headers.OSAndroid,
		"appkey":           "NzNiMDhlYzQyNjJm",
		"lbcookie":         "1",
		"usertype":         "JIO",
	}

	// Make the HTTP request
	resp, err := utils.MakeHTTPRequest(utils.HTTPRequestConfig{
		URL:     CHANNELS_API_URL,
		Method:  "GET",
		Headers: requestHeaders,
	}, client)
	if err != nil {
		utils.Log.Printf("Error fetching channels from JioTV API: %v", err)
		return ChannelsResponse{}, err
	}
	defer fasthttp.ReleaseResponse(resp)

	var apiResponse ChannelsResponse

	// Parse JSON response
	if err := utils.ParseJSONResponse(resp, &apiResponse); err != nil {
		utils.Log.Printf("Error parsing channels API response: %v", err)
		return ChannelsResponse{}, err
	}

	// disable sony channels temporarily
	// apiResponse.Result = append(apiResponse.Result, SONY_CHANNELS_API...)

	// Load and append custom channels if configured
	if config.Cfg.CustomChannelsFile != "" {
		customChannels := getCustomChannels()
		apiResponse.Result = append(apiResponse.Result, customChannels...)
	}

	return apiResponse, nil
}

// FilterChannels Function is used to filter channels by language and category
func FilterChannels(channels []Channel, language, category int) []Channel {
	var filteredChannels []Channel
	for _, channel := range channels {
		// if both language and category is set, then use and operator
		if language != 0 && category != 0 {
			if channel.Language == language && channel.Category == category {
				filteredChannels = append(filteredChannels, channel)
			}
		} else if language != 0 {
			if channel.Language == language {
				filteredChannels = append(filteredChannels, channel)
			}
		} else if category != 0 {
			if channel.Category == category {
				filteredChannels = append(filteredChannels, channel)
			}
		} else {
			filteredChannels = append(filteredChannels, channel)
		}
	}
	return filteredChannels
}

// FilterChannelsByDefaults filters channels by arrays of default categories and languages
// If both arrays are provided, channels must match at least one category AND one language
// If only one array is provided, channels must match at least one item from that array
// If both arrays are empty, all channels are returned
func FilterChannelsByDefaults(channels []Channel, categories, languages []int) []Channel {
	// If both arrays are empty, return all channels
	if len(categories) == 0 && len(languages) == 0 {
		return channels
	}

	// Use maps for efficient O(1) lookups
	categorySet := make(map[int]struct{}, len(categories))
	for _, cat := range categories {
		categorySet[cat] = struct{}{}
	}

	languageSet := make(map[int]struct{}, len(languages))
	for _, lang := range languages {
		languageSet[lang] = struct{}{}
	}

	filteredChannels := make([]Channel, 0, len(channels))
	for _, channel := range channels {
		// If categories are specified, channel must match one of them
		categoryMatch := len(categories) == 0
		if !categoryMatch {
			_, categoryMatch = categorySet[channel.Category]
		}

		// If languages are specified, channel must match one of them
		languageMatch := len(languages) == 0
		if !languageMatch {
			_, languageMatch = languageSet[channel.Language]
		}

		// Include channel if it matches both criteria (or if a criterion is not specified)
		if categoryMatch && languageMatch {
			filteredChannels = append(filteredChannels, channel)
		}
	}
	return filteredChannels
}

func ReplaceM3U8(baseUrl, match []byte, params, channel_id string, quality string) []byte {
	// Attempt to extract hdnea from params if present
	hdnea := ""
	if strings.Contains(params, "hdnea=") {
		// naive extraction without net/url to avoid allocation; safe since params is small
		for _, p := range strings.Split(params, "&") {
			if strings.HasPrefix(p, "hdnea=") {
				hdnea = strings.TrimPrefix(p, "hdnea=")
				break
			}
		}
	}
	config := EncryptedURLConfig{
		BaseURL:     string(baseUrl),
		Match:       string(match),
		Params:      params,
		ChannelID:   channel_id,
		EndpointURL: "/render.m3u8",
		Quality:     quality,
		Hdnea:       hdnea,
	}

	result, err := CreateEncryptedURL(config)
	if err != nil {
		return nil
	}
	return result
}

func ReplaceTS(baseUrl, match []byte, params string) []byte {
	if config.Cfg.DisableTSHandler {
		return []byte(string(baseUrl) + string(match) + "?" + params)
	}

	hdnea := ""
	if strings.Contains(params, "hdnea=") {
		for _, p := range strings.Split(params, "&") {
			if strings.HasPrefix(p, "hdnea=") {
				hdnea = strings.TrimPrefix(p, "hdnea=")
				break
			}
		}
	}
	config := EncryptedURLConfig{
		BaseURL:     string(baseUrl),
		Match:       string(match),
		Params:      params,
		EndpointURL: "/render.ts",
		Hdnea:       hdnea,
	}

	result, err := CreateEncryptedURL(config)
	if err != nil {
		return nil
	}
	return result
}

func ReplaceAAC(baseUrl, match []byte, params string) []byte {
	if config.Cfg.DisableTSHandler {
		return []byte(string(baseUrl) + string(match) + "?" + params)
	}

	hdnea := ""
	if strings.Contains(params, "hdnea=") {
		for _, p := range strings.Split(params, "&") {
			if strings.HasPrefix(p, "hdnea=") {
				hdnea = strings.TrimPrefix(p, "hdnea=")
				break
			}
		}
	}
	config := EncryptedURLConfig{
		BaseURL:     string(baseUrl),
		Match:       string(match),
		Params:      params,
		EndpointURL: "/render.ts",
		Hdnea:       hdnea,
	}

	result, err := CreateEncryptedURL(config)
	if err != nil {
		return nil
	}
	return result
}

func ReplaceKey(match []byte, params, channel_id string) []byte {
	hdnea := ""
	if strings.Contains(params, "hdnea=") {
		for _, p := range strings.Split(params, "&") {
			if strings.HasPrefix(p, "hdnea=") {
				hdnea = strings.TrimPrefix(p, "hdnea=")
				break
			}
		}
	}
	config := EncryptedURLConfig{
		BaseURL:     "",
		Match:       string(match),
		Params:      params,
		ChannelID:   channel_id,
		EndpointURL: "/render.key",
		Hdnea:       hdnea,
	}

	result, err := CreateEncryptedURL(config)
	if err != nil {
		return nil
	}
	return result
}

func getSLChannel(channelID string) (*LiveURLOutput, error) {
	// Check if the channel is available in the SONY_CHANNELS map
	if val, ok := SONY_JIO_MAP[channelID]; ok {
		// If the channel is available in the SONY_CHANNELS map, then return the link
		result := new(LiveURLOutput)

		chu, err := base64.StdEncoding.DecodeString(SONY_CHANNELS[val])
		if err != nil {
			utils.Log.Panic(err)
			return nil, err
		}

		channel_url := string(chu)

		// Make a get request to the channel url and store location header in actual_url
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		req.SetRequestURI(channel_url)
		req.Header.SetMethod("GET")

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		// Perform the HTTP GET request
		if err := utils.GetRequestClient().Do(req, resp); err != nil {
			utils.Log.Panic(err)
		}

		if resp.StatusCode() != fasthttp.StatusFound {
			utils.Log.Panicf("Request failed with status code: %d", resp.StatusCode())
			utils.Log.Panicln("Response: ", string(resp.Body()))
		}

		// Store the location header in actual_url
		actual_url := string(resp.Header.Peek("Location"))

		result.Result = actual_url
		result.Bitrates.Auto = actual_url
		return result, nil
	} else {
		// If the channel is not available in the SONY_CHANNELS map, then return an error
		return nil, fmt.Errorf("Channel not found")
	}
}

func (tv *Television) GetCatchupURL(channelID, srno, start, end string) (*LiveURLOutput, error) {
	formData := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(formData)

	formData.Add("stream_type", "Catchup")
	formData.Add("channel_id", channelID)
	formData.Add("programId", srno)
	formData.Add("showtime", "000000")
	formData.Add("srno", srno)
	formData.Add("begin", start)
	formData.Add("end", end)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	for key, value := range tv.Headers {
		req.Header.Set(key, value)
	}

	url := "https://" + JIOTV_API_DOMAIN + urls.PlaybackAPIPath
	req.Header.Set(headers.AccessToken, tv.AccessToken)
	req.SetRequestURI(url)
	req.Header.SetMethod("POST")
	req.SetBody(formData.QueryString())
	req.Header.Set("channel_id", channelID)
	req.Header.Set("srno", srno)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := tv.Client.Do(req, resp); err != nil {
			if strings.Contains(err.Error(), "server closed connection before returning the first response byte") {
				utils.Log.Printf("Retrying the catchup request (attempt %d/%d)...", i+1, maxRetries)
				continue
			}
			utils.Log.Panicln(err)
			return nil, err
		}
		break
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		response := string(resp.Body())
		utils.Log.Printf("Catchup request failed with status code: %d", resp.StatusCode())
		utils.Log.Println("Request headers:", req.Header.String())
		utils.Log.Println("Request data:", formData.String())
		utils.Log.Printf("API Response: %s", response)
		var errorResp map[string]interface{}
		if err := json.Unmarshal(resp.Body(), &errorResp); err == nil {
			if code, ok := errorResp["code"].(float64); ok {
				utils.Log.Printf("API Error Code: %.0f", code)
			}
			if message, ok := errorResp["message"].(string); ok {
				utils.Log.Printf("API Error Message: %s", message)
			}
		}
		return nil, fmt.Errorf("catchup request failed with status code: %d", resp.StatusCode())
	}

	var result LiveURLOutput
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		utils.Log.Panicln(err)
		return nil, err
	}

	extractHdneaFromURL := func(u string) string {
		if u == "" {
			return ""
		}
		idx := strings.Index(u, "hdnea=")
		if idx == -1 {
			return ""
		}
		token := u[idx+len("hdnea="):]
		if i := strings.IndexByte(token, '&'); i != -1 {
			token = token[:i]
		}
		return token
	}
	hdnea := extractHdneaFromURL(result.Result)
	if hdnea == "" {
		hdnea = extractHdneaFromURL(result.Bitrates.Auto)
	}
	result.Hdnea = hdnea
	return &result, nil
}

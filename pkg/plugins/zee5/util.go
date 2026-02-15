package zee5

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jiotv-go/jiotv_go/v3/pkg/secureurl"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

const (
	USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0"
	// ZEE5_DUMMY_CHANNEL_ID is a placeholder channel used for bootstrapping token generation.
	ZEE5_DUMMY_CHANNEL_ID = "0-9-9z583538"
)

func getMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// generateDDToken generates the 'x-dd-token' header value by Base64 encoding
// a JSON string of device capabilities.
func generateDDToken() (string, error) {
	data := map[string]interface{}{
		"schema_version":   "1",
		"os_name":          "N/A",
		"os_version":       "N/A",
		"platform_name":    "Chrome",
		"platform_version": "104",
		"device_name":      "",
		"app_name":         "Web",
		"app_version":      "2.52.31",
		"player_capabilities": map[string]interface{}{
			"audio_channel": []string{"STEREO"},
			"video_codec":   []string{"H264"},
			"container":     []string{"MP4", "TS"},
			"package":       []string{"DASH", "HLS"},
			"resolution":    []string{"240p", "SD", "HD", "FHD"},
			"dynamic_range": []string{"SDR"},
		},
		"security_capabilities": map[string]interface{}{
			"encryption":              []string{"WIDEVINE_AES_CTR"},
			"widevine_security_level": []string{"L3"},
			"hdcp_version":            []string{"HDCP_V1", "HDCP_V2", "HDCP_V2_1", "HDCP_V2_2"},
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Base64 encode the JSON bytes
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)

	return encoded, nil
}

// generateGuestToken generates a version 4 (random) UUID string.
func generateGuestToken() string {
	return uuid.New().String()
}

// fetchPlatformToken fetches the Zee5 page and extracts the 'gwapiPlatformToken'
// using a regular expression.
func fetchPlatformToken(userAgent string) (string, error) {
	urlStr := "https://www.zee5.com/live-tv/aaj-tak/0-9-aajtak"

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	re := regexp.MustCompile(`"gwapiPlatformToken"\s*:\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(string(bodyBytes))
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("platform token not found in page")
}

// fetchM3u8URL orchestrates the token generation and performs the final API call
// to retrieve the M3U8 video stream URL.
func fetchM3u8URL(guestToken, platformToken, ddToken string, userAgent string) (string, error) {
	// API configuration
	baseURL := "https://spapi.zee5.com/singlePlayback/getDetails/secure"

	// Construct the full URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	q := u.Query()
	q.Set("channel_id", ZEE5_DUMMY_CHANNEL_ID)
	q.Set("device_id", guestToken)
	q.Set("platform_name", "desktop_web")
	q.Set("translation", "en")
	q.Set("user_language", "en,hi,te")
	q.Set("country", "IN")
	q.Set("state", "")
	q.Set("app_version", "4.24.0")
	q.Set("user_type", "guest")
	q.Set("check_parental_control", "false")
	u.RawQuery = q.Encode()
	fullURL := u.String()

	// Payload for the POST request
	payload := map[string]string{
		"x-access-token":   platformToken,
		"X-Z5-Guest-Token": guestToken,
		"x-dd-token":       ddToken,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://www.zee5.com")
	req.Header.Set("referer", "https://www.zee5.com/")
	req.Header.Set("user-agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid response from API, status %d", resp.StatusCode)
	}

	var responseData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return "", fmt.Errorf("json decode error: %w", err)
	}

	// Extract the 'video_token'
	keyOsDetails, ok := responseData["keyOsDetails"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("keyOsDetails missing in response")
	}

	videoToken, ok := keyOsDetails["video_token"].(string)
	if !ok || videoToken == "" {
		return "", fmt.Errorf("video_token missing in response")
	}

	// Simple URL validation check
	if strings.HasPrefix(videoToken, "http") {
		return videoToken, nil
	}
	return "", fmt.Errorf("invalid video_token url")
}

// generateCookieZee5 fetches the M3U8 URL content and extracts the 'hdntl'
// token/cookie from the response body using a regular expression.
func generateCookieZee5(userAgent string) (map[string]string, error) {
	// 1. Get required tokens
	guestToken := generateGuestToken()

	platformToken, err := fetchPlatformToken(userAgent)
	if err != nil {
		return nil, err
	}

	ddToken, err := generateDDToken()
	if err != nil {
		return nil, err
	}

	// 2. Fetch the M3U8 URL
	m3u8URL, err := fetchM3u8URL(guestToken, platformToken, ddToken, userAgent)
	if err != nil {
		return nil, err
	}

	// 3. Fetch the M3U8 content to get the 'hdntl' cookie
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
	req, err := http.NewRequest("GET", m3u8URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create M3U8 content request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching M3U8 content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching M3U8 content, status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read M3U8 content body: %w", err)
	}
	body := string(bodyBytes)

	re := regexp.MustCompile(`hdntl=([^\s"]+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 0 {
		return map[string]string{"cookie": matches[0]}, nil
	}
	return nil, fmt.Errorf("hdntl token not found in response")
}

func transformURL(relURLStr string, baseURL *url.URL, isMaster bool, prefix string) string {
	relURL, err := url.Parse(relURLStr)
	if err != nil {
		return relURLStr
	}

	absURL := baseURL.ResolveReference(relURL).String()
	coded_url, err := secureurl.EncryptURL(absURL)
	if err != nil {
		utils.Log.Println(err)
		return ""
	}
	path := relURL.Path
	if path == "" {
		path = relURL.String()
	}

	// Simple extension check
	isM3U8 := strings.Contains(path, ".m3u8")
	isSegment := strings.Contains(path, ".ts") || strings.Contains(path, ".mp4")
	segmentType := ""
	if strings.Contains(path, ".mp4") {
		segmentType = "mp4"
	} else {
		segmentType = "ts"
	}
	if isM3U8 {
		// Construct new URL
		newParams := url.Values{}

		newParams.Set("auth", coded_url)
		return fmt.Sprintf("%s/zee5/render/playlist.m3u8?%s", prefix, newParams.Encode())

	} else if isSegment && !isMaster {
		// Proxy segments only in Index handler
		newParams := url.Values{}
		newParams.Set("auth", coded_url)
		return fmt.Sprintf("%s/zee5/render/segment.%s?%s", prefix, segmentType, newParams.Encode())
	}

	// Fallback: use absolute URL
	return absURL
}

func fetchContent(targetURL string) ([]byte, http.Header, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("User-Agent", USER_AGENT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	return body, resp.Header, err
}

// handlePlaylist contains the common logic for processing m3u8 playlists
func handlePlaylist(c *fiber.Ctx, isMaster bool, targetURLStr string, prefix string) {
	if targetURLStr == "" {
		c.Status(fiber.StatusBadRequest).SendString("missing url param")
		return
	}

	// Fetch content
	content, _, err := fetchContent(targetURLStr)
	if err != nil {
		c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("failed to fetch: %v", err))
		return
	}

	// Base URL for resolution
	baseURL, err := url.Parse(targetURLStr)
	if err != nil {
		c.Status(fiber.StatusBadRequest).SendString("invalid target url")
		return
	}

	// Process content
	var processedLines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))

	// Regex for EXT-X-MEDIA URI
	reMediaURI := regexp.MustCompile(`URI="([^"]+)"`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			processedLines = append(processedLines, line)
			continue
		}
		if strings.HasPrefix(trimmed, "#EXT-X-MAP") || strings.HasPrefix(trimmed, "#EXT-X-MEDIA") {
			// Handle URI inside EXT-X-MAP or EXT-X-MEDIA
			matches := reMediaURI.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				originalURI := matches[1]
				newURI := transformURL(originalURI, baseURL, isMaster, prefix)
				line = strings.Replace(line, originalURI, newURI, 1)
			}
			processedLines = append(processedLines, line)
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			processedLines = append(processedLines, line)
			continue
		}

		// It's a URI line
		newLine := transformURL(trimmed, baseURL, isMaster, prefix)
		processedLines = append(processedLines, newLine)
	}

	c.Set("Content-Type", "application/vnd.apple.mpegurl")
	c.Set("Access-Control-Allow-Origin", "*") // Good practice for proxy

	c.Send([]byte(strings.Join(processedLines, "\n")))
}

// ProxySegmentHandler handles the /segment.ts endpoint
func ProxySegmentHandler(c *fiber.Ctx) {
	targetURLStr := c.Query("auth")
	if targetURLStr == "" {
		c.Status(fiber.StatusBadRequest).SendString("missing auth param")
		return
	}

	coded_url, err := secureurl.DecryptURL(c.Query("auth"))
	if err != nil {
		c.Status(fiber.StatusBadRequest).SendString("invalid auth param")
		return
	}
	targetURLStr = coded_url

	content, respHeaders, err := fetchContent(targetURLStr)
	if err != nil {
		c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("failed to fetch: %v", err))
		return
	}

	// Copy headers
	if ct := respHeaders.Get("Content-Type"); ct != "" {
		c.Set("Content-Type", ct)
	}
	if cl := respHeaders.Get("Content-Length"); cl != "" {
		c.Set("Content-Length", cl)
	}
	c.Set("Access-Control-Allow-Origin", "*")

	c.Send(content)
}

package zee5

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jiotv-go/jiotv_go/v3/internal/config"
	"github.com/jiotv-go/jiotv_go/v3/pkg/secureurl"
	"github.com/jiotv-go/jiotv_go/v3/pkg/television"
)

var cache *expirable.LRU[string, string]

func init() {
	cache = expirable.NewLRU[string, string](50, nil, time.Second*3600)
}

// Zee5Language is a flexible language field that accepts both integer IDs
// (as used in the upstream data.json) and ISO 639-1 string codes.
type Zee5Language struct {
	intVal int    // set when JSON value is a number
	strVal string // set when JSON value is a string
	isInt  bool
}

// UnmarshalJSON handles both numeric and string language values.
func (l *Zee5Language) UnmarshalJSON(data []byte) error {
	// Try integer first
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		l.intVal = n
		l.isInt = true
		return nil
	}
	// Fall back to string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("zee5 language: cannot unmarshal %s", string(data))
	}
	l.strVal = s
	l.isInt = false
	return nil
}

// MarshalJSON round-trips the original representation.
func (l Zee5Language) MarshalJSON() ([]byte, error) {
	if l.isInt {
		return json.Marshal(l.intVal)
	}
	return json.Marshal(l.strVal)
}

// JioTVID returns the JioTV language ID for this language value.
func (l Zee5Language) JioTVID() int {
	if l.isInt {
		return zee5IntLanguageID(l.intVal)
	}
	return zee5LanguageID(l.strVal)
}

// String returns a human-readable representation.
func (l Zee5Language) String() string {
	if l.isInt {
		return strconv.Itoa(l.intVal)
	}
	return l.strVal
}

type ChannelItem struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	URL      string      `json:"url"`
	Logo     string      `json:"logo"`
	Language Zee5Language `json:"language"`
	Slug     string      `json:"slug"`
	Genre    string      `json:"genre"`
	Chno     string      `json:"chno"`
}

// zee5LangToJioTV maps ISO 639-1 language codes from zee5 data to JioTV language IDs.
var zee5LangToJioTV = map[string]int{
	"hi": 1,  // Hindi
	"mr": 2,  // Marathi
	"pa": 3,  // Punjabi
	"ur": 4,  // Urdu
	"bn": 5,  // Bengali
	"en": 6,  // English
	"ml": 7,  // Malayalam
	"ta": 8,  // Tamil
	"gu": 9,  // Gujarati
	"or": 10, // Odia
	"te": 11, // Telugu
	"bh": 12, // Bhojpuri
	"kn": 13, // Kannada
	"as": 14, // Assamese
	"ne": 15, // Nepali
	"fr": 16, // French
}

// zee5IntToJioTV maps Zee5 integer language IDs (as used in data.json) to JioTV language IDs.
// The Zee5 numbering aligns with JioTV for IDs 1-13; others are mapped explicitly.
var zee5IntToJioTV = map[int]int{
	1:  1,  // Hindi
	2:  2,  // Marathi
	3:  3,  // Punjabi
	4:  4,  // Urdu / Arabic
	5:  5,  // Bengali
	6:  6,  // English
	7:  7,  // Malayalam
	8:  8,  // Tamil
	9:  9,  // Gujarati
	10: 10, // Odia
	11: 11, // Telugu
	12: 12, // Bhojpuri
	13: 13, // Kannada
	14: 14, // Assamese
	15: 15, // Nepali
	16: 16, // French
	18: 18, // Other / Indonesian
}

// zee5LanguageID converts an ISO 639-1 string code to a JioTV language ID.
func zee5LanguageID(code string) int {
	if id, ok := zee5LangToJioTV[strings.ToLower(code)]; ok {
		return id
	}
	return 18 // Other
}

// zee5IntLanguageID converts a Zee5 integer language ID to a JioTV language ID.
func zee5IntLanguageID(id int) int {
	if jioID, ok := zee5IntToJioTV[id]; ok {
		return jioID
	}
	return 18 // Other
}

type DataFile struct {
	Title string        `json:"title"`
	Data  []ChannelItem `json:"data"`
}

func readDataFile() (*DataFile, error) {
	// First, try to get cached data
	cachedData := GetCachedZee5Data()
	if cachedData != nil {
		return cachedData, nil
	}

	// Try to load from configured file path
	return LoadZee5Data(config.Cfg.Zee5DataFile)
}

func LiveHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	id = strings.Replace(id, ".m3u8", "", 1)
	data, err := readDataFile()
	if err != nil {
		c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		return err
	}
	url := ""

	for _, channelItem := range data.Data {
		if channelItem.ID == id {
			url = channelItem.URL
			break
		}
	}
	if url == "" {
		c.Set("ID", id)
		return c.SendString("Channel not found")
	}
	uaHash := getMD5Hash(USER_AGENT)
	cookie, found := cache.Get(uaHash)
	if !found {
		cookieMap, err := generateCookieZee5(USER_AGENT)
		if err != nil {
			c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			return err
		}
		cookie = cookieMap["cookie"]
		cache.Add(uaHash, cookie)
	}
	hostURL := strings.ToLower(c.Protocol()) + "://" + c.Hostname()
	handlePlaylist(c, true, url+"?"+cookie, hostURL)
	return nil
}

func RenderHandler(c *fiber.Ctx) error {
	hostURL := strings.ToLower(c.Protocol()) + "://" + c.Hostname()
	coded_url, err := secureurl.DecryptURL(c.Query("auth"))
	if err != nil {
		return err
	}
	handlePlaylist(c, false, coded_url, hostURL)
	return nil
}

func RenderTSChunkHandler(c *fiber.Ctx) error {
	ProxySegmentHandler(c)
	return nil
}

func RenderMP4ChunkHandler(c *fiber.Ctx) error {
	ProxySegmentHandler(c)
	return nil
}

func RegisterRoutes(app *fiber.App) {
	app.Get("/zee5/:id", LiveHandler)
	app.Get("/zee5/render/playlist.m3u8", RenderHandler)
	app.Get("/zee5/render/segment.ts", RenderTSChunkHandler)
	app.Get("/zee5/render/segment.mp4", RenderMP4ChunkHandler)
}

func GetChannels() []television.Channel {
	data, err := readDataFile()
	channels := []television.Channel{}

	if err != nil || data == nil {
		return channels
	}

	for _, channelItem := range data.Data {
		channels = append(channels, television.Channel{
			ID:       channelItem.ID,
			Name:     channelItem.Name,
			URL:      "zee5/" + channelItem.ID,
			LogoURL:  channelItem.Logo,
			Category: 0,
			Language: channelItem.Language.JioTVID(),
			IsHD:     strings.Contains(strings.ToLower(channelItem.Name), " hd"),
			IsCustom: true,
		})
	}
	return channels
}

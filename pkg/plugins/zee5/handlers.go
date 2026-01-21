package zee5

import (
	"embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jiotv-go/jiotv_go/v3/pkg/secureurl"
	"github.com/jiotv-go/jiotv_go/v3/pkg/television"
)

type ttlEntry struct {
	val     string
	expires time.Time
}

type ttlCache struct {
	items map[string]ttlEntry
	ttl   time.Duration
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{
		items: make(map[string]ttlEntry),
		ttl:   ttl,
	}
}

func (c *ttlCache) Get(key string) (string, bool) {
	e, ok := c.items[key]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expires) {
		delete(c.items, key)
		return "", false
	}
	return e.val, true
}

func (c *ttlCache) Set(key, val string) {
	c.items[key] = ttlEntry{val: val, expires: time.Now().Add(c.ttl)}
}

func init() {
	cookieCache = newTTLCache(time.Hour)
}

var cookieCache *ttlCache

type ChannelItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Logo     string `json:"logo"`
	Language int    `json:"language"`
	Slug     string `json:"slug"`
}

type DataFile struct {
	Title string        `json:"title"`
	Data  []ChannelItem `json:"data"`
}

func readDataFile() (*DataFile, error) {
	b, err := dataFile.ReadFile("data.json")
	if err != nil {
		return nil, err
	}
	var d DataFile
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

//go:embed data.json
var dataFile embed.FS

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
	cookie, found := cookieCache.Get(uaHash)
	if !found {
		cookieMap, err := generateCookieZee5(USER_AGENT)
		if err != nil {
			c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			return err
		}
		cookie = cookieMap["cookie"]
		cookieCache.Set(uaHash, cookie)
	}
	hostURL := c.BaseURL()
	quality := c.Query("q")
	handlePlaylist(c, true, url+"?"+cookie, hostURL, quality)
	return nil
}

func RenderHandler(c *fiber.Ctx) error {
	hostURL := c.BaseURL()
	coded_url, err := secureurl.DecryptURL(c.Query("auth"))
	if err != nil {
		return err
	}
	handlePlaylist(c, false, coded_url, hostURL, "")
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
	if err != nil {
		return nil
	}
	for _, channelItem := range data.Data {
		channels = append(channels, television.Channel{
			ID:       channelItem.ID,
			Name:     channelItem.Name,
			URL:      "zee5/" + channelItem.ID,
			LogoURL:  channelItem.Logo,
			Category: 0,
			Language: channelItem.Language,
			IsHD:     false,
			IsCustom: true,
		})
	}
	return channels
}

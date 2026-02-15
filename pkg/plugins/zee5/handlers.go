package zee5

import (
	"embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jiotv-go/jiotv_go/v3/pkg/secureurl"
	"github.com/jiotv-go/jiotv_go/v3/pkg/television"
)

var cache *expirable.LRU[string, string]

func init() {
	cache = expirable.NewLRU[string, string](50, nil, time.Second*3600)
}

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

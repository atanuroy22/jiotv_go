package handlers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/jiotv-go/jiotv_go/v3/internal/config"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/headers"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/urls"
	internalUtils "github.com/jiotv-go/jiotv_go/v3/internal/utils"
	"github.com/jiotv-go/jiotv_go/v3/pkg/epg"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

const (
	EPG_POSTER_URL = urls.EPGPosterURLSlash
)

var externalEPGMu sync.Mutex
var localEPGMu sync.Mutex

// EPGHandler handles EPG requests
func EPGHandler(c *fiber.Ctx) error {
	epgFilePath := utils.GetPathPrefix() + "epg.xml.gz"
	// if epg.xml.gz exists, return it
	if _, err := os.Stat(epgFilePath); err == nil {
		return c.SendFile(epgFilePath, true)
	}

	if config.Cfg.EPGURL != "" {
		externalEPGMu.Lock()
		err := epg.DownloadExternalEPG(config.Cfg.EPGURL, epgFilePath)
		externalEPGMu.Unlock()
		if err == nil {
			if _, statErr := os.Stat(epgFilePath); statErr == nil {
				return c.SendFile(epgFilePath, true)
			}
		}
		return internalUtils.InternalServerError(c, err.Error())
	}

	if config.Cfg.EPG {
		localEPGMu.Lock()
		defer localEPGMu.Unlock()

		if _, err := os.Stat(epgFilePath); err == nil {
			return c.SendFile(epgFilePath, true)
		}

		if err := epg.GenXMLGz(epgFilePath); err != nil {
			return internalUtils.InternalServerError(c, err.Error())
		}

		if _, err := os.Stat(epgFilePath); err == nil {
			return c.SendFile(epgFilePath, true)
		}
	}

	errMessage := "EPG not found. Enable JIOTV_EPG or set JIOTV_EPG_URL to an external guide."
	utils.Log.Println(errMessage)
	return internalUtils.NotFoundError(c, errMessage)
}

// WebEPGHandler responds to requests for EPG data for individual channels.
func WebEPGHandler(c *fiber.Ctx) error {
	// Get channel ID from URL
	channelID := c.Params("channelID")

	if len(channelID) >= 2 && strings.HasPrefix(channelID, "sl") {
		channelID = channelID[2:]
	}

	channelIntID, err := strconv.Atoi(channelID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid channel ID")
	}

	// Get offset from URL
	offset, err := strconv.Atoi(c.Params("offset"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid offset")
	}

	url := fmt.Sprintf(epg.EPG_URL, offset, channelIntID)
	internalUtils.SetCommonHeaders(c, headers.UserAgentOkHttp)
	if err := proxy.Do(c, url, TV.Client); err != nil {
		return err
	}

	c.Response().Header.Del(fiber.HeaderServer)
	return nil
}

// PosterHandler loads image from JioTV server
func PosterHandler(c *fiber.Ctx) error {
	// catch all params
	url := EPG_POSTER_URL + c.Params("date") + "/" + c.Params("file")
	return internalUtils.ProxyRequest(c, url, TV.Client, "")
}

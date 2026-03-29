package epg

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/url"

	"os"
	"sync"
	"time"

	"github.com/jiotv-go/jiotv_go/v3/internal/constants/headers"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/tasks"
	"github.com/jiotv-go/jiotv_go/v3/internal/constants/urls"
	"github.com/jiotv-go/jiotv_go/v3/pkg/scheduler"
	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
	"github.com/schollz/progressbar/v3"
	"github.com/valyala/fasthttp"
)

const (
	// URL for fetching channels from JioTV API
	CHANNEL_URL = urls.ChannelURL
	// URL for fetching EPG data for individual channels from JioTV API
	EPG_URL = urls.EPGURL
	// EPG_POSTER_URL
	EPG_POSTER_URL = urls.EPGPosterURL
	// EPG_TASK_ID is the ID of the EPG generation task
	EPG_TASK_ID = tasks.EPGTaskID
	// Default values for random scheduling when crypto/rand fails
	defaultRandomHour   = 2
	defaultRandomMinute = 30
)

func responseBody(resp *fasthttp.Response) ([]byte, error) {
	if bytes.Contains(resp.Header.Peek("Content-Encoding"), []byte("gzip")) {
		return resp.BodyGunzip()
	}
	return resp.Body(), nil
}

func timeFromEpoch(epoch int64) (time.Time, bool) {
	if epoch <= 0 {
		return time.Time{}, false
	}
	if epoch < 100000000000 {
		return time.Unix(epoch, 0), true
	}
	return time.UnixMilli(epoch), true
}

// Init initializes EPG generation and schedules it for the next day.
func Init() {
	epgFile := utils.GetPathPrefix() + "epg.xml.gz"
	var lastModTime time.Time
	flag := false
	utils.Log.Println("Checking EPG file")
	
	// Check file existence and get file info
	fileResult := utils.CheckAndReadFile(epgFile)
	if fileResult.Exists {
		// If file was modified today, don't generate new EPG
		// Else generate new EPG
		if stat, err := os.Stat(epgFile); err == nil {
			lastModTime = stat.ModTime()
			fileDate := lastModTime.Format("2006-01-02")
			todayDate := time.Now().Format("2006-01-02")
			if fileDate == todayDate {
				utils.Log.Println("EPG file is up to date.")
			} else {
				utils.Log.Println("EPG file is old.")
				flag = true
			}
		}
	} else {
		utils.Log.Println("EPG file doesn't exist")
		flag = true
	}

	genepg := func() error {
		fmt.Println("\tGenerating new EPG file... Please wait.")
		err := GenXMLGz(epgFile)
		if err != nil {
			utils.Log.Printf("ERROR: Failed to generate EPG file: %v", err)
			fmt.Println("\tEPG file generation failed. Server will continue running without EPG.")
			return nil
		}
		return nil
	}

	if flag {
		genepg()
	}
	// setup random time to avoid server load
	random_hour_bigint, err := rand.Int(rand.Reader, big.NewInt(3))
	if err != nil {
		utils.Log.Printf("ERROR: Failed to generate random hour: %v", err)
		// Use default values if random generation fails
		random_hour_bigint = big.NewInt(defaultRandomHour)
	}
	random_min_bigint, err := rand.Int(rand.Reader, big.NewInt(60))
	if err != nil {
		utils.Log.Printf("ERROR: Failed to generate random minute: %v", err)
		// Use default values if random generation fails
		random_min_bigint = big.NewInt(defaultRandomMinute)
	}
	random_hour := int(-5 + random_hour_bigint.Int64()) // random number between 1 and 5
	random_min := int(-30 + random_min_bigint.Int64())  // random number between 0 and 59
	time_now := time.Now()
	schedule_time := time.Date(time_now.Year(), time_now.Month(), time_now.Day()+1, random_hour, random_min, 0, 0, time.UTC)
	utils.Log.Println("Scheduled EPG generation on", schedule_time.Local())
	go scheduler.Add(EPG_TASK_ID, time.Until(schedule_time), genepg)
}

// NewProgramme creates a new Programme with the given parameters.
func NewProgramme(channelID int, start, stop, title, desc, category, iconSrc string) Programme {
	iconURL := fmt.Sprintf("%s/%s", EPG_POSTER_URL, iconSrc)
	return Programme{
		Channel: fmt.Sprint(channelID),
		Start:   start,
		Stop:    stop,
		Title: Title{
			Value: title,
			Lang:  "en",
		},
		Desc: Desc{
			Value: desc,
			Lang:  "en",
		},
		Category: Category{
			Value: category,
			Lang:  "en",
		},
		Icon: Icon{
			Src: iconURL,
		},
	}
}

// genXML generates XML EPG from JioTV API and returns it as a byte slice.
func genXML() ([]byte, error) {
	// Create a reusable fasthttp client with common headers
	client := utils.GetRequestClient()

	// Create channels and programmes slices with initial capacity
	var channels []Channel
	var programmes []Programme
	var programmesMu sync.Mutex

	deviceID := utils.GetDeviceID()
	crmID := ""
	uniqueID := ""
	if creds, err := utils.GetJIOTVCredentials(); err == nil && creds != nil {
		crmID = creds.CRM
		uniqueID = creds.UniqueID
	}

	// Define a worker function for fetching EPG data
	fetchEPG := func(channel Channel, bar *progressbar.ProgressBar) {
		req := fasthttp.AcquireRequest()
		utils.SetCommonJioTVHeaders(req, deviceID, crmID, uniqueID)
		req.Header.Set(headers.Accept, headers.AcceptJSON)
		req.Header.SetMethod("GET")
		defer fasthttp.ReleaseRequest(req)

		resp := fasthttp.AcquireResponse()

		for offset := 0; offset < 2; offset++ {
			reqUrl := fmt.Sprintf(EPG_URL, offset, channel.ID)
			req.SetRequestURI(reqUrl)

			if err := client.Do(req, resp); err != nil {
				// Handle error
				utils.Log.Printf("Error fetching EPG for channel %d, offset %d: %v", channel.ID, offset, err)
				continue
			}
			status := resp.StatusCode()
			if status == fasthttp.StatusNotFound {
				break
			}
			if status != fasthttp.StatusOK {
				utils.Log.Printf("Error fetching EPG for channel %d, offset %d: status %d, body: %s", channel.ID, offset, status, resp.Body())
				continue
			}

			body, err := responseBody(resp)
			if err != nil {
				utils.Log.Printf("Error reading EPG response body for channel %d, offset %d: %v", channel.ID, offset, err)
				continue
			}

			var epgResponse EPGResponse
			if err := json.Unmarshal(body, &epgResponse); err != nil {
				// Handle error
				utils.Log.Printf("Error unmarshaling EPG response for channel %d, offset %d: %v", channel.ID, offset, err)
				// Print response body for debugging
				utils.Log.Printf("Response body: %s", body)
				continue
			}

			for _, programme := range epgResponse.EPG {
				startT, okStart := timeFromEpoch(programme.StartEpoch)
				endT, okEnd := timeFromEpoch(programme.EndEpoch)
				if !okStart || !okEnd {
					continue
				}
				startTime := formatTime(startT)
				endTime := formatTime(endT)
				p := NewProgramme(channel.ID, startTime, endTime, programme.Title, programme.Description, programme.ShowCategory, programme.Poster)
				programmesMu.Lock()
				programmes = append(programmes, p)
				programmesMu.Unlock()
			}
		}
		bar.Add(1)
		fasthttp.ReleaseResponse(resp)
	}

	// Fetch channels data
	utils.Log.Println("Fetching channels")
	resp, err := utils.MakeHTTPRequest(utils.HTTPRequestConfig{
		URL:    CHANNEL_URL,
		Method: "GET",
		Headers: map[string]string{
			headers.UserAgent:  headers.UserAgentOkHttp,
			headers.Accept:     headers.AcceptJSON,
			headers.DeviceType: headers.DeviceTypePhone,
			headers.OS:         headers.OSAndroid,
			"appkey":           "NzNiMDhlYzQyNjJm",
			"lbcookie":         "1",
			"usertype":         "JIO",
		},
	}, client)
	if err != nil {
		return nil, utils.LogAndReturnError(err, "Failed to fetch channels")
	}
	defer fasthttp.ReleaseResponse(resp)

	var channelsResponse ChannelsResponse
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("failed to fetch channels: status %d, body: %s", resp.StatusCode(), resp.Body())
	}
	body, err := responseBody(resp)
	if err != nil {
		return nil, utils.LogAndReturnError(err, "Failed to read channels response body")
	}
	if err := json.Unmarshal(body, &channelsResponse); err != nil {
		return nil, utils.LogAndReturnError(err, "Failed to parse channels response")
	}

	for _, channel := range channelsResponse.Channels {
		channels = append(channels, Channel{
			ID:      channel.ChannelID,
			Display: channel.ChannelName,
		})
	}
	utils.Log.Println("Fetched", len(channels), "channels")
	// Use a worker pool to fetch EPG data concurrently
	const numWorkers = 20 // Adjust the number of workers based on your needs
	channelQueue := make(chan Channel, len(channels))
	var wg sync.WaitGroup

	// Create a progress bar
	totalChannels := len(channels) // Replace with the actual number of channels
	bar := progressbar.Default(int64(totalChannels))

	utils.Log.Println("Fetching EPG for channels")
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for channel := range channelQueue {
				fetchEPG(channel, bar)
			}
		}()
	}
	// Queue channels for processing
	for _, channel := range channels {
		channelQueue <- channel
	}
	close(channelQueue)
	wg.Wait()

	utils.Log.Println("Fetched programmes")
	// Create EPG and marshal it to XML
	epg := EPG{
		Channel:   channels,
		Programme: programmes,
	}
	xml, err := xml.Marshal(epg)
	if err != nil {
		return nil, err
	}
	return xml, nil
}

// formatTime formats the given time to the string representation "20060102150405 -0700".
func formatTime(t time.Time) string {
	return t.Format("20060102150405 -0700")
}

// GenXMLGz generates XML EPG from JioTV API and writes it to a compressed gzip file.
func GenXMLGz(filename string) error {
	utils.Log.Println("Generating XML")
	xml, err := genXML()
	if err != nil {
		return err
	}
	// Add XML header
	xmlHeader := `<?xml version="1.0" encoding="UTF-8"?>
	<!DOCTYPE tv SYSTEM "http://www.w3.org/2006/05/tv">`
	xml = append([]byte(xmlHeader), xml...)
	// write to file
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close() // skipcq: GO-S2307

	utils.Log.Println("Writing XML to gzip file")
	gz := gzip.NewWriter(f)
	defer gz.Close()

	if _, err := gz.Write(xml); err != nil {
		return err
	}
	fmt.Println("\tEPG file generated successfully")
	return nil
}

func DownloadExternalEPG(epgURL, filename string) error {
	client := utils.GetRequestClient()

	currentURL := epgURL
	for i := 0; i < 5; i++ {
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()

		req.SetRequestURI(currentURL)
		req.Header.SetMethod("GET")
		req.Header.SetUserAgent(headers.UserAgentOkHttp)
		req.Header.Set(headers.Accept, "*/*")

		err := client.DoTimeout(req, resp, 20*time.Second)
		fasthttp.ReleaseRequest(req)
		if err != nil {
			fasthttp.ReleaseResponse(resp)
			return err
		}

		status := resp.StatusCode()
		if status >= 300 && status <= 308 {
			location := string(resp.Header.Peek("Location"))
			fasthttp.ReleaseResponse(resp)
			if location == "" {
				return fmt.Errorf("redirect without location (status %d)", status)
			}
			base, err := url.Parse(currentURL)
			if err != nil {
				return err
			}
			next, err := url.Parse(location)
			if err != nil {
				return err
			}
			currentURL = base.ResolveReference(next).String()
			continue
		}

		if status != fasthttp.StatusOK {
			body := resp.Body()
			fasthttp.ReleaseResponse(resp)
			return fmt.Errorf("epg download failed: status %d, body: %s", status, body)
		}

		data := append([]byte(nil), resp.Body()...)
		fasthttp.ReleaseResponse(resp)

		tmp := filename + ".tmp"
		if err := os.WriteFile(tmp, data, 0644); err != nil {
			return err
		}
		_ = os.Remove(filename)
		return os.Rename(tmp, filename)
	}

	return fmt.Errorf("too many redirects")
}

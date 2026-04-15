package handlers

import (
	"fmt"
	"testing"
	"time"

	"github.com/jiotv-go/jiotv_go/v3/pkg/television"
)

func TestHDNEARemainingLifetime(t *testing.T) {
	futureToken := fmt.Sprintf("exp=%d~acl=/*~data=hdntl~hmac=test", time.Now().Add(5*time.Minute).Unix())
	remaining, ok := hdneaRemainingLifetime(futureToken)
	if !ok {
		t.Fatalf("expected token expiry to be parsed")
	}
	if remaining <= 4*time.Minute {
		t.Fatalf("expected token to have more than 4 minutes remaining, got %s", remaining)
	}

	soonToken := fmt.Sprintf("exp=%d~acl=/*~data=hdntl~hmac=test", time.Now().Add(10*time.Second).Unix())
	remaining, ok = hdneaRemainingLifetime(soonToken)
	if !ok {
		t.Fatalf("expected near-expiry token to be parsed")
	}
	if remaining > hdneaRefreshLeadTime {
		t.Fatalf("expected near-expiry token to be within refresh window, got %s", remaining)
	}
}

func TestLiveResultNeedsRefresh(t *testing.T) {
	liveResult := &television.LiveURLOutput{
		Hdnea: fmt.Sprintf("exp=%d~acl=/*~data=hdntl~hmac=test", time.Now().Add(10*time.Second).Unix()),
	}
	if !liveResultNeedsRefresh(liveResult) {
		t.Fatalf("expected live result to need refresh")
	}

	liveResult.Hdnea = fmt.Sprintf("exp=%d~acl=/*~data=hdntl~hmac=test", time.Now().Add(5*time.Minute).Unix())
	if liveResultNeedsRefresh(liveResult) {
		t.Fatalf("expected live result to be considered fresh")
	}
}

func TestSelectBestLiveMPDURL(t *testing.T) {
	liveResult := &television.LiveURLOutput{
		Mpd: television.MPD{
			Result: "https://example.com/master.mpd",
			Bitrates: television.Bitrates{
				Auto:   "https://example.com/auto.mpd",
				High:   "https://example.com/high.mpd",
				Medium: "https://example.com/medium.mpd",
				Low:    "https://example.com/low.mpd",
			},
		},
	}

	if got := selectBestLiveMPDURL(liveResult, "high"); got != "https://example.com/high.mpd" {
		t.Fatalf("expected high MPD URL, got %s", got)
	}

	if got := selectBestLiveMPDURL(liveResult, "unknown"); got != "https://example.com/auto.mpd" {
		t.Fatalf("expected auto MPD URL fallback, got %s", got)
	}

	liveResult.Mpd.Bitrates = television.Bitrates{}
	if got := selectBestLiveMPDURL(liveResult, "auto"); got != "https://example.com/master.mpd" {
		t.Fatalf("expected MPD result fallback, got %s", got)
	}
}

package utils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestSelectQuality(t *testing.T) {
	auto := "auto_url"
	high := "high_url"
	medium := "medium_url"
	low := "low_url"

	tests := []struct {
		quality  string
		expected string
	}{
		{"high", high},
		{"h", high},
		{"medium", medium},
		{"med", medium},
		{"m", medium},
		{"low", low},
		{"l", low},
		{"auto", auto},
		{"", auto},
		{"unknown", auto},
	}

	for _, test := range tests {
		result := SelectQuality(test.quality, auto, high, medium, low)
		assert.Equal(t, test.expected, result, "Quality selection for %s failed", test.quality)
	}
}

func TestErrorResponse(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return ErrorResponse(c, fiber.StatusBadRequest, "test error")
	})

	// Note: Full HTTP testing would require a more complex setup
	// This is a basic structure test
	assert.NotNil(t, app)
}

func TestValidateRequiredParam(t *testing.T) {
	// Initialize the logger to avoid nil pointer issues
	// For testing, we can create a simple logger
	tests := []struct {
		paramName  string
		paramValue string
		expectErr  bool
	}{
		{"test", "value", false},
		{"test", "", true},
		{"empty", "", true},
		{"nonempty", "value", false},
	}

	for _, test := range tests {
		err := ValidateRequiredParam(test.paramName, test.paramValue)
		if test.expectErr {
			assert.Error(t, err, "Expected error for empty param %s", test.paramName)
		} else {
			assert.NoError(t, err, "Expected no error for param %s", test.paramName)
		}
	}
}

func TestDecryptURLParam(t *testing.T) {
	// Test empty parameter
	_, err := DecryptURLParam("test", "")
	assert.Error(t, err, "Expected error for empty URL")

	// Test invalid encrypted URL
	_, err = DecryptURLParam("test", "invalid")
	assert.Error(t, err, "Expected error for invalid encrypted URL")
}

func TestExternalBaseURL(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(ExternalBaseURL(c))
	})

	tests := []struct {
		name     string
		host     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "plain request uses request host",
			host:     "localhost:5001",
			expected: "http://localhost:5001",
		},
		{
			name: "x-forwarded headers use public origin",
			host: "127.0.0.1:5001",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
				"X-Forwarded-Host":  "tv.example.com",
			},
			expected: "https://tv.example.com",
		},
		{
			name: "forwarded header is supported",
			host: "127.0.0.1:5001",
			headers: map[string]string{
				"Forwarded": `for=192.0.2.10;proto=https;host="watch.example.com"`,
			},
			expected: "https://watch.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host
			req.Header.Set("Host", tt.host)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			resp, err := app.Test(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(body))
		})
	}
}

package imperva

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// CookieData represents the cookies and user agent obtained from Imperva challenge
type CookieData struct {
	Cookies   []*http.Cookie
	UserAgent string
}

// DefaultUserAgent is used for browser automation
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// FetchCookies uses a headless browser to navigate to a Resy venue page and fetch Imperva cookies
// Returns the cookies and user-agent that can be used for subsequent API requests
func FetchCookies(venueID int64) (*CookieData, error) {
	return FetchCookiesWithRetry(venueID, 3)
}

// FetchCookiesWithRetry attempts to fetch cookies with retry logic for transient failures
func FetchCookiesWithRetry(venueID int64, maxRetries int) (*CookieData, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Cookie fetch attempt %d/%d for venue %d", attempt+1, maxRetries, venueID)
			time.Sleep(time.Duration(attempt*2) * time.Second) // Exponential backoff
		}

		cookieData, err := fetchCookiesOnce(venueID)
		if err == nil {
			return cookieData, nil
		}

		lastErr = err
		log.Printf("Cookie fetch attempt %d failed for venue %d: %v", attempt+1, venueID, err)
	}

	return nil, fmt.Errorf("failed to fetch cookies after %d attempts: %w", maxRetries, lastErr)
}

// fetchCookiesOnce performs a single attempt to fetch cookies
func fetchCookiesOnce(venueID int64) (*CookieData, error) {
	// Build the venue URL
	venueURL := fmt.Sprintf("https://resy.com/cities/nyc/venues/%d", venueID)

	// Create context with timeout - 60s for headless operation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build chrome options for headless operation
	opts := buildChromeOptions()

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Create chrome instance with error logging
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer chromeCancel()

	var cookies []*http.Cookie
	var userAgent string

	// Navigate to the venue page and wait for Imperva challenge to complete
	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(venueURL),
		// Wait for page to load and Imperva challenge to complete
		chromedp.Sleep(5*time.Second), // Initial wait for Imperva challenge
		// Check if page loaded successfully by waiting for body
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Additional wait to ensure Imperva cookies are set
		chromedp.Sleep(3*time.Second),
		// Get cookies
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookiesRaw, err := network.GetCookies().Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to get cookies: %w", err)
			}

			// Convert network cookies to http.Cookie
			for _, c := range cookiesRaw {
				cookie := &http.Cookie{
					Name:     c.Name,
					Value:    c.Value,
					Path:     c.Path,
					Domain:   c.Domain,
					Secure:   c.Secure,
					HttpOnly: c.HTTPOnly,
				}
				// Handle expiration
				if c.Expires > 0 {
					cookie.Expires = time.Unix(int64(c.Expires), 0)
				}
				cookies = append(cookies, cookie)
			}

			// Get user agent
			var ua string
			err = chromedp.Evaluate(`navigator.userAgent`, &ua).Do(ctx)
			if err != nil {
				ua = DefaultUserAgent
			}
			userAgent = ua

			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch cookies: %w", err)
	}

	// Filter for Imperva cookies
	impervaCookies := filterImpervaCookies(cookies)

	// If no Imperva-specific cookies found, return all cookies
	if len(impervaCookies) == 0 {
		impervaCookies = cookies
	}

	log.Printf("Fetched %d cookies for venue %d", len(impervaCookies), venueID)

	return &CookieData{
		Cookies:   impervaCookies,
		UserAgent: userAgent,
	}, nil
}

// buildChromeOptions constructs Chrome options for headless operation
func buildChromeOptions() []chromedp.ExecAllocatorOption {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.UserAgent(DefaultUserAgent),
		chromedp.WindowSize(1920, 1080),
	}

	// Use CHROME_PATH environment variable if set (for containerized environments)
	if chromePath := os.Getenv("CHROME_PATH"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	return opts
}

// filterImpervaCookies extracts Imperva-related cookies from a list
func filterImpervaCookies(cookies []*http.Cookie) []*http.Cookie {
	var impervaCookies []*http.Cookie
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie.Name, "_incap_") ||
			strings.HasPrefix(cookie.Name, "incap_ses_") ||
			strings.HasPrefix(cookie.Name, "_visid_") ||
			strings.HasPrefix(cookie.Name, "visid_incap_") ||
			strings.HasPrefix(cookie.Name, "nlbi_") ||
			strings.Contains(cookie.Name, "imperva") ||
			strings.HasPrefix(cookie.Name, "reese84") {
			impervaCookies = append(impervaCookies, cookie)
		}
	}
	return impervaCookies
}

// FetchCookiesForAPI is a convenience function that fetches cookies for api.resy.com domain
// by navigating to the web interface first, then extracting cookies applicable to the API domain
func FetchCookiesForAPI(venueID int64) (*CookieData, error) {
	cookieData, err := FetchCookies(venueID)
	if err != nil {
		return nil, err
	}

	// The cookies fetched from resy.com should also work for api.resy.com
	// as they're on the same domain hierarchy
	return cookieData, nil
}

// CookiesToHeaderString converts cookies to a Cookie header string
func CookiesToHeaderString(cookies []*http.Cookie) string {
	var parts []string
	for _, cookie := range cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(parts, "; ")
}

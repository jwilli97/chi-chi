package imperva

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

// FetchCookies uses a headless browser to navigate to a Resy venue page and fetch Imperva cookies
// Returns the cookies and user-agent that can be used for subsequent API requests
func FetchCookies(venueID int64) (*CookieData, error) {
	// Build the venue URL
	venueURL := fmt.Sprintf("https://resy.com/cities/nyc/venues/%d", venueID)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create chrome instance options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create chrome instance
	ctx, cancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	var cookies []*http.Cookie
	var userAgent string

	// Navigate to the venue page and wait for Imperva challenge to complete
	err := chromedp.Run(ctx,
		chromedp.Navigate(venueURL),
		// Wait for page to load and Imperva challenge to complete
		// We wait for either the page to fully load or a timeout
		chromedp.Sleep(5*time.Second), // Initial wait for Imperva challenge
		// Check if page loaded successfully by waiting for body or specific element
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
			// Include all cookies - Imperva cookies might be on various domains
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
				// Fallback to default if we can't get it
				ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
			}
			userAgent = ua

			return nil
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch cookies: %w", err)
	}

	// Filter for Imperva cookies (typically start with _incap_ or _visid_)
	var impervaCookies []*http.Cookie
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie.Name, "_incap_") ||
			strings.HasPrefix(cookie.Name, "incap_ses_") ||
			strings.HasPrefix(cookie.Name, "_visid_") ||
			strings.HasPrefix(cookie.Name, "visid_incap_") ||
			strings.HasPrefix(cookie.Name, "nlbi_") ||
			strings.Contains(cookie.Name, "imperva") {
			impervaCookies = append(impervaCookies, cookie)
		}
	}

	// If no Imperva-specific cookies found, return all cookies
	if len(impervaCookies) == 0 {
		impervaCookies = cookies
	}

	log.Printf("Fetched %d cookies for venue %d, user-agent: %s", len(impervaCookies), venueID, userAgent)

	return &CookieData{
		Cookies:   impervaCookies,
		UserAgent: userAgent,
	}, nil
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

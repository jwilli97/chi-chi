package store

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// CookieData represents stored cookie information for a venue
type CookieData struct {
	Cookies   []*http.Cookie `json:"cookies"`
	UserAgent string         `json:"user_agent"`
	ExpiresAt time.Time      `json:"expires_at"`
}

// SaveCookies stores cookies for a venue with a TTL
func SaveCookies(ctx context.Context, venueID int64, cookies []*http.Cookie, userAgent string, ttl time.Duration) error {
	data := CookieData{
		Cookies:   cookies,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(ttl),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return GetClient().Set(ctx, CookieKey(venueID), jsonData, ttl).Err()
}

// GetCookies retrieves cookies for a venue
func GetCookies(ctx context.Context, venueID int64) (*CookieData, error) {
	jsonData, err := GetClient().Get(ctx, CookieKey(venueID)).Bytes()
	if err != nil {
		return nil, err
	}

	var data CookieData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

// DeleteCookies removes cookies for a venue
func DeleteCookies(ctx context.Context, venueID int64) error {
	return GetClient().Del(ctx, CookieKey(venueID)).Err()
}

// CookieExists checks if cookies exist for a venue
func CookieExists(ctx context.Context, venueID int64) (bool, error) {
	result, err := GetClient().Exists(ctx, CookieKey(venueID)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// GetCookieTTL returns the remaining TTL for a venue's cookies
func GetCookieTTL(ctx context.Context, venueID int64) (time.Duration, error) {
	return GetClient().TTL(ctx, CookieKey(venueID)).Result()
}




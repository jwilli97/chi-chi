package store

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	client *redis.Client
	once   sync.Once
)

// GetClient returns the singleton Redis client
func GetClient() *redis.Client {
	once.Do(func() {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379"
		}

		redisPassword := os.Getenv("REDIS_PASSWORD")

		client = redis.NewClient(&redis.Options{
			Addr:     redisURL,
			Password: redisPassword,
			DB:       0,
		})
	})
	return client
}

// Ping checks if Redis is connected
func Ping(ctx context.Context) error {
	return GetClient().Ping(ctx).Err()
}

// Close closes the Redis connection
func Close() error {
	if client != nil {
		return client.Close()
	}
	return nil
}

// Key prefixes
const (
	CookieKeyPrefix      = "cookies:"
	ReservationKeyPrefix = "reservations:"
	PendingSetKey        = "reservations:pending"
)

// CookieKey returns the Redis key for a venue's cookies
func CookieKey(venueID int64) string {
	return fmt.Sprintf("%s%d", CookieKeyPrefix, venueID)
}

// ReservationKey returns the Redis key for a reservation
func ReservationKey(id string) string {
	return fmt.Sprintf("%s%s", ReservationKeyPrefix, id)
}




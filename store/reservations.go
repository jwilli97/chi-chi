package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ScheduledReservation represents a reservation scheduled for future execution
type ScheduledReservation struct {
	ID               string    `json:"id"`
	VenueID          int64     `json:"venue_id"`
	ReservationTime  time.Time `json:"reservation_time"`
	PartySize        int       `json:"party_size"`
	TablePreferences []string  `json:"table_preferences"`
	AuthToken        string    `json:"auth_token"`
	RunTime          time.Time `json:"run_time"` // When to attempt the reservation
	CreatedAt        time.Time `json:"created_at"`
}

// SaveReservation stores a scheduled reservation in Redis
func SaveReservation(ctx context.Context, res *ScheduledReservation) error {
	jsonData, err := json.Marshal(res)
	if err != nil {
		return err
	}

	// Store the reservation data
	key := ReservationKey(res.ID)
	if err := GetClient().Set(ctx, key, jsonData, 0).Err(); err != nil {
		return err
	}

	// Add to the pending sorted set with RunTime as score for efficient polling
	score := float64(res.RunTime.Unix())
	return GetClient().ZAdd(ctx, PendingSetKey, redis.Z{
		Score:  score,
		Member: res.ID,
	}).Err()
}

// GetReservation retrieves a reservation by ID
func GetReservation(ctx context.Context, id string) (*ScheduledReservation, error) {
	jsonData, err := GetClient().Get(ctx, ReservationKey(id)).Bytes()
	if err != nil {
		return nil, err
	}

	var res ScheduledReservation
	if err := json.Unmarshal(jsonData, &res); err != nil {
		return nil, err
	}

	return &res, nil
}

// DeleteReservation removes a reservation from Redis
func DeleteReservation(ctx context.Context, id string) error {
	// Remove from sorted set
	if err := GetClient().ZRem(ctx, PendingSetKey, id).Err(); err != nil {
		return err
	}

	// Remove the reservation data
	return GetClient().Del(ctx, ReservationKey(id)).Err()
}

// GetPendingReservations returns reservations that are due to run (RunTime <= now)
func GetPendingReservations(ctx context.Context) ([]*ScheduledReservation, error) {
	now := float64(time.Now().Unix())

	// Get all reservation IDs with RunTime <= now
	ids, err := GetClient().ZRangeByScore(ctx, PendingSetKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err != nil {
		return nil, err
	}

	reservations := make([]*ScheduledReservation, 0, len(ids))
	for _, id := range ids {
		res, err := GetReservation(ctx, id)
		if err != nil {
			// Log but continue - reservation might have been deleted
			continue
		}
		reservations = append(reservations, res)
	}

	return reservations, nil
}

// GetNextReservation returns the earliest pending reservation
func GetNextReservation(ctx context.Context) (*ScheduledReservation, error) {
	// Get the first (earliest) reservation ID from the sorted set
	ids, err := GetClient().ZRange(ctx, PendingSetKey, 0, 0).Result()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil // No pending reservations
	}

	return GetReservation(ctx, ids[0])
}

// GetAllPendingReservations returns all scheduled reservations (for status endpoint)
func GetAllPendingReservations(ctx context.Context) ([]*ScheduledReservation, error) {
	// Get all reservation IDs from the sorted set
	ids, err := GetClient().ZRange(ctx, PendingSetKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	reservations := make([]*ScheduledReservation, 0, len(ids))
	for _, id := range ids {
		res, err := GetReservation(ctx, id)
		if err != nil {
			continue
		}
		reservations = append(reservations, res)
	}

	return reservations, nil
}

// CountPendingReservations returns the number of pending reservations
func CountPendingReservations(ctx context.Context) (int64, error) {
	return GetClient().ZCard(ctx, PendingSetKey).Result()
}

// GenerateReservationID creates a unique ID for a reservation
func GenerateReservationID() string {
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}




package redisModel

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

/**
 * SORTED SET OPERATIONS
 * Redis sorted sets for time-indexed data (e.g., last-seen timestamps).
 */

// ZAdd adds or updates a member in a sorted set with the given score.
// The key is automatically prefixed using the RedisClient's keyPrefix.
func (r *RedisClient) ZAdd(ctx context.Context, key string, score float64, member string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.ZAdd(ctx, prefixed, redis.Z{Score: score, Member: member}).Err()
}

// ZScore returns the score of a member in a sorted set.
// Returns redis.Nil if the member does not exist.
func (r *RedisClient) ZScore(ctx context.Context, key string, member string) (float64, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.ZScore(ctx, prefixed, member).Result()
}

// ZMScore returns the scores of multiple members in a sorted set in a single round-trip.
// Returns NaN for members that do not exist.
func (r *RedisClient) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.ZMScore(ctx, prefixed, members...).Result()
}

// ZRangeByScore returns members in a sorted set with scores between min and max.
// Supports pagination via offset and count.
func (r *RedisClient) ZRangeByScore(ctx context.Context, key string, min string, max string, offset int64, count int64) ([]string, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.ZRangeByScore(ctx, prefixed, &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  count,
	}).Result()
}

// ZRem removes a member from a sorted set.
func (r *RedisClient) ZRem(ctx context.Context, key string, member string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.ZRem(ctx, prefixed, member).Err()
}

/**
 * HASH OPERATIONS
 * Redis hashes for key-value pairs per key (e.g., miss counters per asset).
 */

// HIncrBy increments a hash field by the given amount and returns the new value.
func (r *RedisClient) HIncrBy(ctx context.Context, key string, field string, incr int64) (int64, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.HIncrBy(ctx, prefixed, field, incr).Result()
}

// HDel removes a field from a hash.
func (r *RedisClient) HDel(ctx context.Context, key string, field string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.HDel(ctx, prefixed, field).Err()
}

/**
 * SET OPERATIONS
 * Redis sets for membership tracking (e.g., alerted assets, active orgs).
 */

// SAdd adds a member to a set.
func (r *RedisClient) SAdd(ctx context.Context, key string, member string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.SAdd(ctx, prefixed, member).Err()
}

// SRem removes a member from a set.
func (r *RedisClient) SRem(ctx context.Context, key string, member string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.SRem(ctx, prefixed, member).Err()
}

// SIsMember checks if a member exists in a set.
func (r *RedisClient) SIsMember(ctx context.Context, key string, member string) (bool, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.SIsMember(ctx, prefixed, member).Result()
}

// SMIsMember checks if multiple members exist in a set in a single round-trip.
// Returns a bool slice in the same order as the input members.
func (r *RedisClient) SMIsMember(ctx context.Context, key string, members ...interface{}) ([]bool, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.SMIsMember(ctx, prefixed, members...).Result()
}

// SMembers returns all members of a set.
func (r *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	prefixed := r.keyPrefix + ":" + key
	return r.client.SMembers(ctx, prefixed).Result()
}

/**
 * PIPELINE OPERATIONS
 * Batch operations for atomic multi-command execution.
 */

// PipelineRemoveFromCollections removes a member from a sorted set, hash, and set atomically.
// Used for cleanup operations (e.g., removing an asset from all health monitoring data).
func (r *RedisClient) PipelineRemoveFromCollections(ctx context.Context, zsetKey string, hashKey string, setKey string, member string) error {
	pipe := r.client.Pipeline()
	pipe.ZRem(ctx, r.keyPrefix+":"+zsetKey, member)
	pipe.HDel(ctx, r.keyPrefix+":"+hashKey, member)
	pipe.SRem(ctx, r.keyPrefix+":"+setKey, member)
	_, err := pipe.Exec(ctx)
	return err
}

/**
 * HELPER FUNCTIONS
 * Conversion utilities for sorted set scores.
 */

// ScoreToTime converts a sorted set score (Unix timestamp) to time.Time.
// Returns nil if the score is 0 or NaN.
func ScoreToTime(score float64) *time.Time {
	if score == 0 || math.IsNaN(score) {
		return nil
	}
	t := time.Unix(int64(score), 0)
	return &t
}

// TimeToScore converts a time.Time to a sorted set score (Unix timestamp).
func TimeToScore(t time.Time) float64 {
	return float64(t.Unix())
}

// ScoresToTimeMap converts a slice of scores to a map of member → *time.Time.
// Members with score 0 or NaN are excluded.
func ScoresToTimeMap(members []string, scores []float64) map[string]*time.Time {
	result := make(map[string]*time.Time, len(members))
	for i, member := range members {
		if i < len(scores) {
			t := ScoreToTime(scores[i])
			if t != nil {
				result[member] = t
			}
		}
	}
	return result
}

// BoolSliceToMap converts a bool slice from SMIsMember to a map of member → bool.
func BoolSliceToMap(members []string, flags []bool) map[string]bool {
	result := make(map[string]bool, len(members))
	for i, member := range members {
		if i < len(flags) {
			result[member] = flags[i]
		}
	}
	return result
}

// FormatCutoff formats a time as a string score for ZRangeByScore max parameter.
func FormatCutoff(t time.Time) string {
	return fmt.Sprintf("%d", t.Unix())
}

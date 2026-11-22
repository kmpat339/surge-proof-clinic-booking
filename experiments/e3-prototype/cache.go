package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheKey = "slots:open"

// returns the list of open slots and where it came from
// with cache or store, using the cache-aside pattern which is to check redis first
// when theres a miss on the cache, we read the store and save to ttl
func getOpenSlots(ctx context.Context, rdb *redis.Client, store *SlotStore, ttl time.Duration) ([]int, string) {
	// ttl of 0 means no cache. means always read the store (baseline)
	if ttl == 0 {
		return store.ListOpen(), "store"
	}

	// 1. try the cache
	// look for a cached copy of the slot list in redis
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		// we have a hit on cache
		var ids []int
		// convert json back into list
		if json.Unmarshal([]byte(cached), &ids) == nil {
			// hit so return cache
			return ids, "cache"
		}
		// case 2. miss or broken redis
	} else if err != redis.Nil {
		log.Printf("redis get failed, falling back to store: %v", err)
	}

	// 2. cache miss. read the store then save the answer for next time
	ids := store.ListOpen()
	data, _ := json.Marshal(ids)
	if err := rdb.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		log.Printf("redis set failed: %v", err)
	}
	return ids, "store"
}
package main

import "sync/atomic"

// counters for the e3 experiments
type Metrics struct {
	requests 	atomic.Int64
	cacheHits	atomic.Int64
	cacheMisses	atomic.Int64
	staleResponses	atomic.Int64
	staleSlots	atomic.Int64
	slotsShown	atomic.Int64
}

// returns how many slots in a cached list are booked in the store
// uses IsOpen so that checking does not count as db read
func countStale(store *SlotStore, ids []int) int64 {
	var stale int64
	for _, id := range ids {
		if !store.IsOpen(id) {
			stale++
		}
	}
	return stale
}


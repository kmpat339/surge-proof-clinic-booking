package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"math/rand"
	"time"
	"flag"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// fake slot store
// map is fake database table. 
// mutex needed for locks
// atomic for atomic counter
type SlotStore struct {
	mu sync.RWMutex
	open map[int]bool
	reads atomic.Int64
}

func NewSlotStore(n int) *SlotStore {
	// creates empty map
	open := make(map[int]bool, n)
	// adds slot IDs 0 through n - 1, and they are all set to true
	for id := 0; id < n; id++ {
		open[id] = true
	}
	// returns pointer to the store
	return &SlotStore{open: open}
}

func(s *SlotStore) ListOpen() []int {
	// add 1 to reads. counts call as database read
	s.reads.Add(1)
	// sleep about 20 s. acts as database call
	time.Sleep(20 * time.Millisecond)
	// take the read lock
	s.mu.RLock()
	defer s.mu.RUnlock()
	// an empty slice with room for every slot
	ids := make([]int, 0, len(s.open))
	// loop over map and append every open slots id to new slice
	for id, isOpen := range s.open {
		if isOpen {
			ids = append(ids, id)
		}
	}
	return ids
}

// books a slot
func (s *SlotStore) Book(id int) bool {
	// write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// if slot isnt open, return booking as failed
	if !s.open[id] {
		return false
	}
	// if open, make false after booking
	s.open[id] = false;
	// booking successful
	return true;
}

// returns a slot to the pool (like a cancellation or expired hold)
func (s *SlotStore) Release(id int) {
	// write lock
	s.mu.Lock()
	defer s.mu.Unlock()
	// make slot open again
	s.open[id] = true
}

func (s *SlotStore) IsOpen(id int) bool {
	// read lock
	s.mu.RLock()
	s.mu.RUnlock()
	// return if open or not
	return s.open[id]
}

func (s *SlotStore) Reads() int64 {
	// reads atomic counter value
	return s.reads.Load()
}

// simulates booking traffic. for every tick it picks a random slot
// books if open or release it if taken
// so slots keep changing
func runBooker(store *SlotStore, numSlots int, every time.Duration) {
	// a clock that sends a signal every on its channel ticker.C
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	// loops once per tick forever
	for range ticker.C {
		// random slot id
		id := rand.Intn(numSlots)
		// try to book. if already taken release
		if !store.Book(id) {
			store.Release(id)
		}
	}
}

const numSlots = 100

func main() {
	// make TTL configurable in a command setting. 0 is default
	ttl := flag.Duration("ttl", 0, "cache TTL. 1s, 10s, 60s. 0 disables the cache")
	//  make booking speed configurable in command setting. 0 is default
	bookEvery := flag.Duration("book-every", 100*time.Millisecond, "how often the booker books or relases a slot. 0 is deafult")
	// read command line flags
	flag.Parse()

	// returns pointer to value
	fmt.Println("cache TTL:", *ttl)
	fmt.Println("book-every:", *bookEvery)

	// creates store with 100 open slots
	store := NewSlotStore(100)

	metrics := &Metrics{}

	// start the background booker so slots keep changing while users browse
	if *bookEvery > 0 {
		go runBooker(store, numSlots, *bookEvery)
	}

	// create redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// check it can reach redis before starting server
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("cannot reach redis: %v", err)
	}
	fmt.Println("connected to redis")
	
	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", })
	})

	router.GET("/slots", func(c *gin.Context) {
		// asks db for open slots
		ids, source := getOpenSlots(c.Request.Context(), rdb, store, *ttl)

		// record metrics in e3 experiments
		metrics.requests.Add(1)
		// if answer comes from redis count as a cache hit
		if source == "cache" {
			metrics.cacheHits.Add(1)
			metrics.slotsShown.Add(int64(len(ids)))
			// is the cached answer out of date?
			if stale := countStale(store, ids); stale > 0 {
				metrics.staleResponses.Add(1)
				metrics.staleSlots.Add(stale)
			}
			// if it came from store count as cache miss
			} else {
				metrics.cacheMisses.Add(1)
			}

		c.JSON(http.StatusOK, gin.H{
			"open": ids, 
			"count": len(ids),
			"source": source,
		})
	})

	router.GET("/metrics", func(c *gin.Context) {
		requests := metrics.requests.Load()
		hits := metrics.cacheHits.Load()
		staleResp := metrics.staleResponses.Load()
		staleSlots := metrics.staleSlots.Load()
		slotsShown := metrics.slotsShown.Load()

		hitRate, staleRate := 0.0, 0.0
		if requests > 0 {
			// share of requests served from the cache
			hitRate = float64(hits) / float64(requests)
			// share of requests that showed at least one booked slot
			staleRate = float64(staleResp) / float64(requests)
		}

		staleSlotFraction := 0.0
		if slotsShown > 0 {
			// share of cached slots shown that were actually booked.
			// the chance a user clicks a slot thats already taken
			staleSlotFraction = float64(staleSlots) / float64(slotsShown)
		}

		c.JSON(http.StatusOK, gin.H{
			"requests":	requests,
			"cache_hits": hits,
			"cache_misses": metrics.cacheMisses.Load(),
			"store_reads": store.Reads(),
			"stale_responses": staleResp,
			"stale_slots": staleSlots,
			"slots_shown": slotsShown,
			"hit_rate": hitRate, 
			"stale_rate": staleRate,
			"stale_slot_fraction": staleSlotFraction,
		})
	})

	router.Run(":8080")
}
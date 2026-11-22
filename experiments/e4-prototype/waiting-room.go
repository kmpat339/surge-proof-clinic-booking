package main

import (
	"math/rand"
	"sync"
	"time"
)

// holds users in a queue and admits few at a time
// waiting means tickets still in line
// joinedAt is when each ticket joined
// admitted is tickets let in
type WaitingRoom struct {
	mu	sync.Mutex
	nextID	int64
	waiting 	[]int64
	joinedAt	map[int64]time.Time
	admitted	map[int64]bool

	// status updated under mu
	admittedCount	int64
	totalWait	time.Duration
	maxWait	time.Duration
}

func NewWaitingRoom() *WaitingRoom {
	return &WaitingRoom{
		joinedAt: make(map[int64]time.Time),
		admitted: 	make(map[int64]bool),
	}
}

// gives user a ticket and puts them in line
func (w *WaitingRoom) Join() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	// create id for user
	w.nextID++
	id := w.nextID
	// append their id 
	w.waiting = append(w.waiting, id)
	// joined time
	w.joinedAt[id] = time.Now()
	return id
}

// reports whether a ticket has been letin
func (w *WaitingRoom) IsAdmitted(id int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.admitted[id]
}

// lets in up to n random tickets from the line
func (w *WaitingRoom) admit(n int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i := 0; i < n && len(w.waiting) > 0; i++ {
		// pick a random ticket then remove it by swapping in the last one
		j := rand.Intn(len(w.waiting))
		id := w.waiting[j]
		last := len(w.waiting) - 1
		w.waiting[j] = w.waiting[last]
		w.waiting = w.waiting[:last]

		w.admitted[id] = true

		wait := time.Since(w.joinedAt[id])
		w.admittedCount++
		w.totalWait += wait
		if wait > w.maxWait {
			w.maxWait = wait
		}
	}
}

// admits perTick tickets for 'every' forever
func (w *WaitingRoom) Run(perTick int, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		w.admit(perTick)
	}
}

func (w *WaitingRoom) Stats() (inLine int, admitted int64, avgWaitMs, maxWaitMs float64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.admittedCount > 0 {
		avgWaitMs = float64(w.totalWait.Milliseconds()) / float64(w.admittedCount)
	}
	return len(w.waiting), w.admittedCount, avgWaitMs, float64(w.maxWait.Milliseconds())
}



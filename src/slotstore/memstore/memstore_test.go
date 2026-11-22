package memstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kmpat339/surge-proof-clinic-booking/src/slotstore"
)

const patients = 100 // how many patients race for the same slot

// oneSlot is the single popular slot everyone wants (E2's "1 popular slot" case).
var oneSlot = []slotstore.Slot{{
	ClinicID: "clinic-1",
	SlotID:   "2026-12-01T10:30",
	Type:     "covid-vaccine",
	Status:   slotstore.StatusOpen,
}}

// holdFunc is the Hold method of whichever store is being tested.
type holdFunc func(ctx context.Context, clinicID, slotID, patientID, idempotencyKey string) (slotstore.Hold, error)

// race releases n patients at the same moment against one slot.
// Input: the store's Hold, number of patients. Output: how many got a hold.
func race(t *testing.T, hold holdFunc, n int) int {
	t.Helper()
	start := make(chan struct{}) // closed once to release every goroutine together
	var wg sync.WaitGroup
	var winners atomic.Int64

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // wait here until all patients are lined up
			_, err := hold(context.Background(), oneSlot[0].ClinicID, oneSlot[0].SlotID,
				fmt.Sprintf("patient-%d", i), fmt.Sprintf("key-%d", i))
			switch {
			case err == nil:
				winners.Add(1) // this patient was told "you got the slot"
			case !errors.Is(err, slotstore.ErrSlotTaken):
				t.Errorf("patient %d: unexpected error %v", i, err)
			}
		}(i)
	}

	close(start) // go!
	wg.Wait()
	return int(winners.Load())
}

// The naive store should tell many patients they got the same slot.
func TestNaiveHoldDoubleBooks(t *testing.T) {
	naive := NewNaive(oneSlot, time.Millisecond) // ~1 ms between read and write
	winners := race(t, naive.Hold, patients)
	t.Logf("naive: %d of %d patients were told they got the slot (%d double-bookings)",
		winners, patients, winners-1)
	if winners <= 1 {
		t.Errorf("expected the naive store to double-book, got %d winner(s)", winners)
	}
}

// The atomic store must give the slot to exactly one patient.
func TestAtomicHoldOneWinner(t *testing.T) {
	atomicStore := NewAtomic(oneSlot)
	winners := race(t, atomicStore.Hold, patients)
	t.Logf("atomic: %d of %d patients were told they got the slot", winners, patients)
	if winners != 1 {
		t.Errorf("expected exactly 1 winner, got %d", winners)
	}
}

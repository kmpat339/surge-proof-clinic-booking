// Package memstore holds two tiny in-memory versions of Hold, used only by
// E2's preliminary race test: a naive one (read, then write) and an atomic one
// (check and write in one step). They are not full slot stores.
package memstore

import (
	"context"
	"sync"
	"time"

	"github.com/kmpat339/surge-proof-clinic-booking/src/slotstore"
)

// store is the shared slot table both versions use.
type store struct {
	mu    sync.Mutex                 // guards every read/write of slots
	slots map[string]*slotstore.Slot // keyed by "clinic/slot"
}

// newStore copies the given slots into a fresh table.
func newStore(slots []slotstore.Slot) *store {
	s := &store{slots: make(map[string]*slotstore.Slot, len(slots))}
	for _, sl := range slots {
		sl := sl // copy so each map entry gets its own slot
		s.slots[key(sl.ClinicID, sl.SlotID)] = &sl
	}
	return s
}

// key builds the map key for one slot, e.g. "clinic-1/10:30".
func key(clinicID, slotID string) string { return clinicID + "/" + slotID }

// Naive holds a slot the way a careless server would: one call to read the
// status, a gap, then a second call to write "held" without checking again.
type Naive struct {
	*store               // embedded so the two versions share the same slot table
	Gap    time.Duration // time between the read and the write (a database round trip)
}

// NewNaive builds a naive store over the given slots.
// Input: slots to seed, and the read-to-write gap. Output: the store.
func NewNaive(slots []slotstore.Slot, gap time.Duration) *Naive {
	return &Naive{store: newStore(slots), Gap: gap}
}

// Hold reads the slot, waits Gap, then writes "held" without re-checking.
// Input: clinic, slot, patient, idempotency key (ignored here).
// Output: a Hold receipt, or ErrNotFound / ErrSlotTaken if the read saw it taken.
func (n *Naive) Hold(ctx context.Context, clinicID, slotID, patientID, idempotencyKey string) (slotstore.Hold, error) {
	// step 1: read the status (its own short critical section, like one DB call)
	n.mu.Lock()
	s, ok := n.slots[key(clinicID, slotID)]
	open := ok && s.Status == slotstore.StatusOpen
	n.mu.Unlock()

	if !ok {
		return slotstore.Hold{}, slotstore.ErrNotFound
	}
	if !open {
		return slotstore.Hold{}, slotstore.ErrSlotTaken
	}

	// the gap: other requests can read "open" before anyone writes "held"
	time.Sleep(n.Gap)

	// step 2: write "held" without checking again, so the last writer silently wins
	until := time.Now().Add(slotstore.HoldDuration)
	n.mu.Lock()
	s.Status = slotstore.StatusHeld
	s.HeldBy = patientID
	s.HoldUntil = until
	n.mu.Unlock()

	return slotstore.Hold{ClinicID: clinicID, SlotID: slotID, PatientID: patientID, ExpiresAt: until}, nil
}

// Atomic holds a slot by checking and writing in one step, the in-memory
// stand-in for DynamoDB's conditional write ("set held only if still open").
type Atomic struct {
	*store // embedded so the two versions share the same slot table
}

// NewAtomic builds an atomic store over the given slots.
// Input: slots to seed. Output: the store.
func NewAtomic(slots []slotstore.Slot) *Atomic {
	return &Atomic{store: newStore(slots)}
}

// Hold checks that the slot is open and marks it held in one critical section.
// Input: clinic, slot, patient, idempotency key (ignored here).
// Output: a Hold receipt, or ErrNotFound / ErrSlotTaken.
func (a *Atomic) Hold(ctx context.Context, clinicID, slotID, patientID, idempotencyKey string) (slotstore.Hold, error) {
	until := time.Now().Add(slotstore.HoldDuration)

	// one lock around both the check and the claim, so no one can sneak in between
	a.mu.Lock()
	defer a.mu.Unlock() // unlock on every return path below

	s, ok := a.slots[key(clinicID, slotID)]
	if !ok {
		return slotstore.Hold{}, slotstore.ErrNotFound
	}
	if s.Status != slotstore.StatusOpen {
		return slotstore.Hold{}, slotstore.ErrSlotTaken // someone already claimed it
	}

	// still holding the lock: claim the slot for this patient
	s.Status = slotstore.StatusHeld
	s.HeldBy = patientID
	s.HoldUntil = until

	return slotstore.Hold{ClinicID: clinicID, SlotID: slotID, PatientID: patientID, ExpiresAt: until}, nil
}

// Package slotstore defines the contract every slot store implements
// (DynamoDB, Raft, and the in-memory store used in tests).
// The booking service, cache and waiting room only ever talk to this interface.
package slotstore

import (
	"context"
	"errors"
	"time"
)

// HoldDuration is how long a picked slot stays reserved for the patient to confirm.
const HoldDuration = 5 * time.Minute

// Status is where a slot is in its life: open -> held -> booked (or back to open).
type Status string

const (
	StatusOpen   Status = "open"
	StatusHeld   Status = "held"   // reserved for one patient until HoldUntil
	StatusBooked Status = "booked" // confirmed, never given to anyone else
)

// Slot is one bookable appointment time at one clinic.
type Slot struct {
	ClinicID  string    // which clinic (DynamoDB partition key)
	SlotID    string    // unique within the clinic, e.g. "2026-12-01T10:30"
	Type      string    // appointment type, e.g. "covid-vaccine", "flu-shot", "checkup"
	Start     time.Time // when the appointment starts
	Status    Status
	HeldBy    string    // patient who holds or booked it; empty while open
	HoldUntil time.Time // when an unconfirmed hold expires; zero unless held
}

// Hold is the receipt from a successful Hold call; Confirm and Release need it.
type Hold struct {
	ClinicID  string
	SlotID    string
	PatientID string
	ExpiresAt time.Time
}

// Errors every store returns, so callers can check them with errors.Is
// no matter which store is underneath.
var (
	ErrNotFound       = errors.New("slot not found")
	ErrSlotTaken      = errors.New("slot is not open") // someone else holds or booked it
	ErrHoldExpired    = errors.New("hold expired or belongs to someone else")
	ErrPatientHasHold = errors.New("patient already has an active hold") // one active hold per patient
)

// Store is the slot-store interface.
type Store interface {
	// ListOpen returns the open slots of one appointment type at a clinic.
	// Input: clinicID, apptType. Output: open slots (may be empty), or an error.
	ListOpen(ctx context.Context, clinicID, apptType string) ([]Slot, error)

	// Hold reserves a slot for HoldDuration, only if it is still open.
	// Input: clinic, slot, patient, and a client-made idempotency key
	// (a retry with the same key returns the first result instead of holding again).
	// Output: the Hold receipt, or ErrNotFound / ErrSlotTaken / ErrPatientHasHold.
	Hold(ctx context.Context, clinicID, slotID, patientID, idempotencyKey string) (Hold, error)

	// Confirm turns a live hold into a booking.
	// Input: the Hold receipt. Output: nil, or ErrHoldExpired if the hold ran out
	// or no longer belongs to this patient.
	Confirm(ctx context.Context, h Hold) error

	// Release gives a held slot back to the pool early.
	// Input: the Hold receipt. Output: nil, or ErrHoldExpired if it was no longer held.
	Release(ctx context.Context, h Hold) error
}

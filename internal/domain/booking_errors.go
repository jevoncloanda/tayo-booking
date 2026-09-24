package domain

import "errors"

var (
	ErrSeatUnavailable          = errors.New("seat unavailable")
	ErrFromStopNotOnRoute       = errors.New("from_stop_id does not belong to this trip's route")
	ErrToStopNotOnRoute         = errors.New("to_stop_id does not belong to this trip's route")
	ErrInvalidSegment           = errors.New("from_stop_id must come before to_stop_id on the route")
	ErrSeatNotOnBus             = errors.New("seat does not belong to this trip's bus")
	ErrBookingNotFound          = errors.New("booking not found")
	ErrForbidden                = errors.New("forbidden")
	ErrBookingAlreadyCancelled  = errors.New("booking already cancelled")
	ErrConfirmedBooking         = errors.New("confirmed booking cannot be cancelled by user")
	ErrCancellationWindowPassed = errors.New("cancellation window has passed")
	ErrBookingStatusUnchanged   = errors.New("booking already has this status")
	ErrIdempotencyConflict      = errors.New("idempotency key was already used for a different booking request")
)

// Package notification defines notifications sent by the calendar scheduler.
package notification

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidNotification means that a notification does not contain all required data.
var ErrInvalidNotification = errors.New("invalid notification")

// Notification contains the event data required to notify its owner.
type Notification struct {
	EventID string    `json:"eventId"`
	Title   string    `json:"title"`
	EventAt time.Time `json:"eventAt"`
	UserID  string    `json:"userId"`
}

// Validate checks the required notification fields.
func (n Notification) Validate() error {
	switch {
	case strings.TrimSpace(n.EventID) == "":
		return fmt.Errorf("%w: event ID is empty", ErrInvalidNotification)
	case strings.TrimSpace(n.Title) == "":
		return fmt.Errorf("%w: title is empty", ErrInvalidNotification)
	case n.EventAt.IsZero():
		return fmt.Errorf("%w: event time is zero", ErrInvalidNotification)
	case strings.TrimSpace(n.UserID) == "":
		return fmt.Errorf("%w: user ID is empty", ErrInvalidNotification)
	default:
		return nil
	}
}

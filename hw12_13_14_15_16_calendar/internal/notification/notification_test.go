package notification

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNotificationValidate(t *testing.T) {
	eventAt := time.Date(2026, time.August, 24, 12, 30, 0, 0, time.UTC)
	valid := Notification{
		EventID: "event-1",
		Title:   "Team meeting",
		EventAt: eventAt,
		UserID:  "user-1",
	}

	tests := []struct {
		name   string
		change func(*Notification)
		valid  bool
	}{
		{name: "valid", valid: true},
		{name: "empty event ID", change: func(value *Notification) { value.EventID = " " }},
		{name: "empty title", change: func(value *Notification) { value.Title = "\t" }},
		{name: "zero event time", change: func(value *Notification) { value.EventAt = time.Time{} }},
		{name: "empty user ID", change: func(value *Notification) { value.UserID = "\n" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := valid
			if tt.change != nil {
				tt.change(&value)
			}

			err := value.Validate()
			if tt.valid {
				if err != nil {
					t.Fatalf("Validate() returned an unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidNotification) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrInvalidNotification)
			}
		})
	}
}

func TestNotificationJSON(t *testing.T) {
	value := Notification{
		EventID: "event-1",
		Title:   "Team meeting",
		EventAt: time.Date(2026, time.August, 24, 12, 30, 0, 0, time.UTC),
		UserID:  "user-1",
	}

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	const want = `{"eventId":"event-1","title":"Team meeting","eventAt":"2026-08-24T12:30:00Z","userId":"user-1"}`
	if string(payload) != want {
		t.Fatalf("JSON = %s, want %s", payload, want)
	}

	var decoded Notification
	if err = json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if decoded != value {
		t.Fatalf("decoded notification = %#v, want %#v", decoded, value)
	}
}

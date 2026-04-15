package domain

import "time"

type EventType string

const (
	EventIdentityRegistered  EventType = "identity.registered"
	EventIdentityLoggedIn    EventType = "identity.logged_in"
	EventIdentityDeactivated EventType = "identity.deactivated"
)

type Event struct {
	ID          string
	Type        EventType
	AggregateID string
	Payload     any
	OccurredAt  time.Time
}

// Payloads

type IdentityRegisteredPayload struct {
	IdentityID string `json:"identity_id"`
	Email      string `json:"email"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

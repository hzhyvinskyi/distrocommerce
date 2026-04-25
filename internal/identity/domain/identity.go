package domain

import (
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Value Objects

type IdentityID uuid.UUID

func NewIdentityID() IdentityID {
	return IdentityID(uuid.New())
}

func (id IdentityID) String() string {
	return uuid.UUID(id).String()
}

func (id IdentityID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 3 || len(raw) > 60 {
		return Email{}, ErrInvalidEmail
	}

	parts := strings.Split(raw, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Email{}, ErrInvalidEmail
	}

	if !strings.Contains(parts[1], ".") {
		return Email{}, ErrInvalidEmail
	}

	return Email{value: strings.ToLower(raw)}, nil
}

func (e Email) String() string {
	return e.value
}

type PasswordHash struct {
	value string
}

func HashPassword(plain string) (PasswordHash, error) {
	if err := validatePasswordStrength(plain); err != nil {
		return PasswordHash{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return PasswordHash{}, err
	}

	return PasswordHash{value: string(hash)}, nil
}

func validatePasswordStrength(plain string) error {
	if len(plain) < 8 {
		return ErrWeakPassword
	}

	var hasLetter, hasDigit bool

	for _, c := range plain {
		if unicode.IsLetter(c) {
			hasLetter = true
		}
		if unicode.IsDigit(c) {
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}

	return nil
}

func (h PasswordHash) Matches(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(h.value), []byte(plain)) == nil
}

func (h PasswordHash) String() string {
	return h.value
}

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Aggregate Root

type Identity struct {
	id           IdentityID
	email        Email
	passwordHash PasswordHash
	roles        []Role
	isActive     bool
	lastLoginAt  *time.Time
	createdAt    time.Time
	updatedAt    time.Time

	events []Event
}

func NewIdentity(email Email, passwordHash PasswordHash) (*Identity, error) {
	now := time.Now()
	i := &Identity{
		id:           IdentityID(uuid.New()),
		email:        email,
		passwordHash: passwordHash,
		roles:        []Role{RoleUser},
		isActive:     true,
		createdAt:    now,
		updatedAt:    now,
	}

	i.addEvent(EventIdentityRegistered, IdentityRegisteredPayload{
		IdentityID: i.id.String(),
		Email:      i.email.String(),
	})

	return i, nil
}

// Getters

func (i Identity) ID() IdentityID             { return i.id }
func (i Identity) Email() Email               { return i.email }
func (i Identity) PasswordHash() PasswordHash { return i.passwordHash }
func (i Identity) Roles() []Role              { return i.roles }
func (i Identity) IsActive() bool             { return i.isActive }
func (i Identity) LastLogin() *time.Time      { return i.lastLoginAt }
func (i Identity) CreatedAt() time.Time       { return i.createdAt }
func (i Identity) UpdatedAt() time.Time       { return i.updatedAt }

// Business Methods

// Domain Events

func (i Identity) Events() []Event {
	events := i.events
	i.events = nil //nolint:staticcheck

	return events
}

func (i Identity) addEvent(t EventType, payload any) {
	i.events = append(i.events, Event{ //nolint:staticcheck
		ID:          uuid.NewString(),
		Type:        t,
		AggregateID: i.id.String(),
		Payload:     payload,
		OccurredAt:  time.Now().UTC(),
	})
}

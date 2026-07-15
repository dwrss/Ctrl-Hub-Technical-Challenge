package domain

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type User struct {
	id   uuid.UUID
	name string
}

func NewUser(id uuid.UUID, name string) (User, error) {
	if name == "" {
		return User{}, fmt.Errorf("%w: name must not be empty", ErrInvalidInput)
	}
	return User{id: id, name: name}, nil
}

func (u User) ID() uuid.UUID { return u.id }
func (u User) Name() string  { return u.name }

type UserRepository interface {
	Get(ctx context.Context, id uuid.UUID) (User, error)
	// GetMany returns the users matching the given IDs, keyed by ID. IDs
	// with no matching user are simply absent from the result, not an
	// error — callers that require every ID to resolve check for that.
	GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error)
}

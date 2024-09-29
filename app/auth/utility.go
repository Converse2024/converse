package auth

import "github.com/google/uuid"

func generateUUID() uuid.UUID {
	return uuid.New()
}

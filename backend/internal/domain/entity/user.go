package entity

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           string          `json:"id"`
	Email        string          `json:"email"`
	PasswordHash string          `json:"-"`
	FullName     string          `json:"fullName"`
	AvatarURL    *string         `json:"avatarUrl"`
	Attributes   json.RawMessage `json:"attributes"` // Dynamic RLS attributes (branch_id, team, etc.)
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	PlanTier  string    `json:"planTier"`
	CreatedAt time.Time `json:"createdAt"`
}

type WorkspaceMember struct {
	WorkspaceID string    `json:"workspaceId"`
	UserID      string    `json:"userId"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
}

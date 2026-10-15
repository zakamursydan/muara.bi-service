package response

import (
	"encoding/json"
	"muara.bi/core/internal/domain/entity"
)

type UserResponse struct {
	ID         string          `json:"id"`
	Email      string          `json:"email"`
	FullName   string          `json:"fullName"`
	AvatarURL  *string         `json:"avatarUrl"`
	Attributes json.RawMessage `json:"attributes"`
}

type AuthResponse struct {
	Token      string             `json:"token"`
	User       UserResponse       `json:"user"`
	Workspaces []entity.Workspace `json:"workspaces"`
}

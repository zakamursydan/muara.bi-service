package request

type RegisterRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	FullName      string `json:"fullName"`
	WorkspaceName string `json:"workspaceName"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

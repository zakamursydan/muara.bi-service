package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

type AuthServiceProvider interface {
	Register(ctx context.Context, req request.RegisterRequest) (*response.AuthResponse, error)
	Login(ctx context.Context, req request.LoginRequest) (*response.AuthResponse, error)
	GetProfile(ctx context.Context, userID string) (*response.UserResponse, []entity.Workspace, error)
}

type AuthService struct {
	userRepo pgsql.UserRepositoryProvider
	cfg      *config.Config
	logger   *logging.Logger
}

func NewAuthService(
	userRepo pgsql.UserRepositoryProvider,
	cfg *config.Config,
	logger *logging.Logger,
) AuthServiceProvider {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
		logger:   logger,
	}
}

type JWTClaims struct {
	UserID     string          `json:"userId"`
	Email      string          `json:"email"`
	Attributes json.RawMessage `json:"attributes"`
	jwt.RegisteredClaims
}

func (s *AuthService) generateToken(user *entity.User) (string, error) {
	claims := JWTClaims{
		UserID:     user.ID,
		Email:      user.Email,
		Attributes: user.Attributes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "muara.bi",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.AppSecretKey))
}

func (s *AuthService) Register(ctx context.Context, req request.RegisterRequest) (*response.AuthResponse, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		return nil, customerror.NewBadRequestError("Email, password, and full name are required", nil)
	}

	existing, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("Database error checking existing user", "error", err)
		return nil, customerror.NewInternalError("Database error", err)
	}
	if existing != nil {
		return nil, customerror.NewConflictError("User with this email already exists", nil)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, customerror.NewInternalError("Failed to hash password", err)
	}

	user := entity.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		Attributes:   json.RawMessage("{}"),
	}

	if err := s.userRepo.CreateUser(ctx, &user); err != nil {
		s.logger.Error("Failed to create user", "error", err)
		return nil, customerror.NewInternalError("Failed to create user", err)
	}

	wsName := req.WorkspaceName
	if wsName == "" {
		wsName = fmt.Sprintf("%s's Workspace", user.FullName)
	}
	wsSlug := strings.ToLower(strings.ReplaceAll(wsName, " ", "-"))

	workspace := entity.Workspace{
		Name:     wsName,
		Slug:     fmt.Sprintf("%s-%d", wsSlug, time.Now().Unix()%10000),
		PlanTier: "pro",
	}

	if err := s.userRepo.CreateWorkspace(ctx, &workspace); err != nil {
		s.logger.Error("Failed to create initial workspace", "error", err)
		return nil, customerror.NewInternalError("Failed to create workspace", err)
	}

	member := entity.WorkspaceMember{
		WorkspaceID: workspace.ID,
		UserID:      user.ID,
		Role:        "OWNER",
	}
	if err := s.userRepo.AddWorkspaceMember(ctx, &member); err != nil {
		s.logger.Error("Failed to add workspace owner", "error", err)
	}

	token, err := s.generateToken(&user)
	if err != nil {
		return nil, customerror.NewInternalError("Failed to generate token", err)
	}

	return &response.AuthResponse{
		Token: token,
		User: response.UserResponse{
			ID:         user.ID,
			Email:      user.Email,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Attributes: user.Attributes,
		},
		Workspaces: []entity.Workspace{workspace},
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req request.LoginRequest) (*response.AuthResponse, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return nil, customerror.NewBadRequestError("Email and password are required", nil)
	}

	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("Database error during login", "error", err)
		return nil, customerror.NewInternalError("Database error", err)
	}
	if user == nil {
		return nil, customerror.NewUnauthorizedError("Invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, customerror.NewUnauthorizedError("Invalid email or password")
	}

	workspaces, err := s.userRepo.GetUserWorkspaces(ctx, user.ID)
	if err != nil {
		s.logger.Warn("Failed to query user workspaces", "error", err)
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, customerror.NewInternalError("Failed to generate token", err)
	}

	return &response.AuthResponse{
		Token: token,
		User: response.UserResponse{
			ID:         user.ID,
			Email:      user.Email,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Attributes: user.Attributes,
		},
		Workspaces: workspaces,
	}, nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID string) (*response.UserResponse, []entity.Workspace, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, customerror.NewInternalError("Database error", err)
	}
	if user == nil {
		return nil, nil, customerror.NewNotFoundError("User not found", errors.New("user does not exist"))
	}

	workspaces, err := s.userRepo.GetUserWorkspaces(ctx, user.ID)
	if err != nil {
		s.logger.Warn("Failed to query user workspaces", "error", err)
	}

	userRes := response.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
		FullName:   user.FullName,
		AvatarURL:  user.AvatarURL,
		Attributes: user.Attributes,
	}

	return &userRes, workspaces, nil
}

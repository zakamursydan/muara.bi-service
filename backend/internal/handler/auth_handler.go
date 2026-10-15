package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"muara.bi/core/internal/dto/request"
	appMiddleware "muara.bi/core/internal/middleware"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/httputil"
	"muara.bi/core/pkg/logging"
)

type AuthHandler struct {
	authService service.AuthServiceProvider
	jwtSecret   string
	logger      *logging.Logger
}

func NewAuthHandler(
	authService service.AuthServiceProvider,
	jwtSecret string,
	logger *logging.Logger,
) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtSecret:   jwtSecret,
		logger:      logger,
	}
}

func (h *AuthHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)

		// Protected route
		r.Group(func(protected chi.Router) {
			protected.Use(appMiddleware.JWTMiddleware(h.jwtSecret))
			protected.Get("/me", h.GetMe)
		})
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req request.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request body", err.Error())
		return
	}

	res, err := h.authService.Register(r.Context(), req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	httputil.WriteCreatedResponse(w, r, "User registered successfully", res)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req request.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteBadRequestResponse(w, r, "Invalid request body", err.Error())
		return
	}

	res, err := h.authService.Login(r.Context(), req)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Login successful", res)
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := appMiddleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		httputil.WriteUnauthorizedResponse(w, r, "Unauthorized context")
		return
	}

	user, workspaces, err := h.authService.GetProfile(r.Context(), userID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	httputil.WriteSuccessResponse(w, r, "Profile retrieved successfully", map[string]any{
		"user":       user,
		"workspaces": workspaces,
	})
}

func (h *AuthHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *customerror.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case customerror.ErrBadRequest:
			httputil.WriteBadRequestResponse(w, r, appErr.Message, nil)
		case customerror.ErrUnauthorized:
			httputil.WriteUnauthorizedResponse(w, r, appErr.Message)
		case customerror.ErrForbidden:
			httputil.WriteForbiddenResponse(w, r, appErr.Message)
		case customerror.ErrNotFound:
			httputil.WriteNotFoundResponse(w, r, appErr.Message)
		case customerror.ErrConflict:
			httputil.WriteBadRequestResponse(w, r, appErr.Message, "conflict")
		default:
			httputil.WriteInternalServerErrorResponse(w, r, appErr.Message, nil)
		}
		return
	}

	httputil.WriteInternalServerErrorResponse(w, r, "Internal server error", err.Error())
}

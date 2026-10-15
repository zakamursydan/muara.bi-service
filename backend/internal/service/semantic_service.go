package service

import (
	"context"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/repository/pgsql"
)

type SemanticServiceProvider interface {
	Create(ctx context.Context, wsID, projID, userID string, req request.CreateSemanticModelRequest) (*response.SemanticModelResponse, error)
	List(ctx context.Context, wsID, projID string) ([]response.SemanticModelResponse, error)
	Get(ctx context.Context, wsID, id string) (*response.SemanticModelResponse, error)
	Update(ctx context.Context, wsID, id string, req request.UpdateSemanticModelRequest) error
	Delete(ctx context.Context, wsID, id string) error
}

type SemanticService struct {
	repo pgsql.SemanticRepositoryProvider
}

func NewSemanticService(repo pgsql.SemanticRepositoryProvider) *SemanticService {
	return &SemanticService{repo: repo}
}

func (s *SemanticService) Create(ctx context.Context, wsID, projID, userID string, req request.CreateSemanticModelRequest) (*response.SemanticModelResponse, error) {
	if req.Name == "" {
		return nil, errors.New("model name is required")
	}

	// Validate SQL query through guardrails
	if err := duckdb.ValidateQuery(req.RawSQL); err != nil {
		return nil, fmt.Errorf("invalid semantic model sql: %w", err)
	}

	model := &entity.SemanticModel{
		WorkspaceID: wsID,
		ProjectID:   projID,
		Name:        req.Name,
		Description: req.Description,
		RawSQL:      req.RawSQL,
		CreatedBy:   &userID,
	}

	if err := s.repo.CreateSemanticModel(ctx, model); err != nil {
		return nil, fmt.Errorf("failed to create semantic model: %w", err)
	}

	return &response.SemanticModelResponse{
		ID:          model.ID,
		WorkspaceID: model.WorkspaceID,
		ProjectID:   model.ProjectID,
		Name:        model.Name,
		Description: model.Description,
		RawSQL:      model.RawSQL,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func (s *SemanticService) List(ctx context.Context, wsID, projID string) ([]response.SemanticModelResponse, error) {
	models, err := s.repo.ListSemanticModelsByProject(ctx, wsID, projID)
	if err != nil {
		return nil, err
	}

	var results []response.SemanticModelResponse
	for _, m := range models {
		results = append(results, response.SemanticModelResponse{
			ID:           m.ID,
			WorkspaceID:  m.WorkspaceID,
			ProjectID:    m.ProjectID,
			Name:         m.Name,
			Description:  m.Description,
			RawSQL:       m.RawSQL,
			CachedSchema: m.CachedSchema,
			CreatedAt:    m.CreatedAt,
			UpdatedAt:    m.UpdatedAt,
		})
	}

	return results, nil
}

func (s *SemanticService) Get(ctx context.Context, wsID, id string) (*response.SemanticModelResponse, error) {
	m, err := s.repo.GetSemanticModelByID(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("semantic model not found")
	}

	return &response.SemanticModelResponse{
		ID:           m.ID,
		WorkspaceID:  m.WorkspaceID,
		ProjectID:    m.ProjectID,
		Name:         m.Name,
		Description:  m.Description,
		RawSQL:       m.RawSQL,
		CachedSchema: m.CachedSchema,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}, nil
}

func (s *SemanticService) Update(ctx context.Context, wsID, id string, req request.UpdateSemanticModelRequest) error {
	if err := duckdb.ValidateQuery(req.RawSQL); err != nil {
		return fmt.Errorf("invalid semantic model sql: %w", err)
	}

	model := &entity.SemanticModel{
		ID:          id,
		WorkspaceID: wsID,
		Name:        req.Name,
		Description: req.Description,
		RawSQL:      req.RawSQL,
	}

	return s.repo.UpdateSemanticModel(ctx, model)
}

func (s *SemanticService) Delete(ctx context.Context, wsID, id string) error {
	return s.repo.DeleteSemanticModel(ctx, wsID, id)
}

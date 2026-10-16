package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/helper/dberr"
	"muara.bi/core/internal/helper/sqlident"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
)

var projectRoles = map[string]struct{}{
	entity.ProjectRoleAdmin:  {},
	entity.ProjectRoleEditor: {},
	entity.ProjectRoleViewer: {},
}

var rlsOperators = map[string]struct{}{
	"=": {}, "!=": {}, ">": {}, "<": {}, ">=": {}, "<=": {},
}

type ProjectServiceProvider interface {
	Create(ctx context.Context, wsID, userID string, req request.CreateProjectRequest) (*response.ProjectResponse, error)
	List(ctx context.Context, wsID string) ([]response.ProjectResponse, error)
	Get(ctx context.Context, wsID, projID string) (*response.ProjectResponse, error)
	ListMembers(ctx context.Context, wsID, projID string) ([]response.ProjectMemberResponse, error)
	AssignMember(ctx context.Context, wsID, projID string, req request.AssignProjectMemberRequest) (*response.ProjectMemberResponse, error)
}

type ProjectService struct {
	repo pgsql.ProjectRepositoryProvider
}

func NewProjectService(repo pgsql.ProjectRepositoryProvider) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(ctx context.Context, wsID, userID string, req request.CreateProjectRequest) (*response.ProjectResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, customerror.NewBadRequestError("project name is required", nil)
	}

	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugify(name)
	}
	if slug == "" {
		return nil, customerror.NewBadRequestError("project slug could not be derived from name", nil)
	}

	project := &entity.Project{
		WorkspaceID: wsID,
		Name:        name,
		Slug:        slug,
		Description: req.Description,
		Icon:        req.Icon,
	}
	if err := s.repo.CreateProject(ctx, project); err != nil {
		if dberr.IsUniqueViolation(err) {
			return nil, customerror.NewConflictError("a project with this slug already exists in the workspace", err)
		}
		return nil, customerror.NewInternalError("failed to create project", err)
	}

	// The creator becomes the project administrator by default.
	member := &entity.ProjectMember{
		ProjectID: project.ID,
		UserID:    userID,
		Role:      entity.ProjectRoleAdmin,
		RLSRules:  json.RawMessage("[]"),
	}
	if err := s.repo.UpsertProjectMember(ctx, member); err != nil {
		return nil, customerror.NewInternalError("failed to assign project creator as admin", err)
	}

	return projectResponse(project), nil
}

func (s *ProjectService) List(ctx context.Context, wsID string) ([]response.ProjectResponse, error) {
	projects, err := s.repo.ListProjects(ctx, wsID)
	if err != nil {
		return nil, err
	}

	results := make([]response.ProjectResponse, 0, len(projects))
	for i := range projects {
		results = append(results, *projectResponse(&projects[i]))
	}
	return results, nil
}

func (s *ProjectService) Get(ctx context.Context, wsID, projID string) (*response.ProjectResponse, error) {
	project, err := s.requireProject(ctx, wsID, projID)
	if err != nil {
		return nil, err
	}
	return projectResponse(project), nil
}

func (s *ProjectService) ListMembers(ctx context.Context, wsID, projID string) ([]response.ProjectMemberResponse, error) {
	if _, err := s.requireProject(ctx, wsID, projID); err != nil {
		return nil, err
	}

	members, err := s.repo.ListProjectMembers(ctx, projID)
	if err != nil {
		return nil, err
	}

	results := make([]response.ProjectMemberResponse, 0, len(members))
	for i := range members {
		results = append(results, *memberResponse(&members[i]))
	}
	return results, nil
}

func (s *ProjectService) AssignMember(ctx context.Context, wsID, projID string, req request.AssignProjectMemberRequest) (*response.ProjectMemberResponse, error) {
	if _, err := s.requireProject(ctx, wsID, projID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, customerror.NewBadRequestError("user_id is required", nil)
	}

	role := strings.ToUpper(strings.TrimSpace(req.Role))
	if _, ok := projectRoles[role]; !ok {
		return nil, customerror.NewBadRequestError("role must be one of ADMIN, EDITOR, VIEWER", nil)
	}

	rules := req.RLSRules
	if rules == nil {
		rules = []entity.RLSRuleSpec{}
	}
	if err := validateRLSRules(rules); err != nil {
		return nil, err
	}

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, customerror.NewBadRequestError("failed to serialize rls rules", err)
	}

	member := &entity.ProjectMember{
		ProjectID: projID,
		UserID:    req.UserID,
		Role:      role,
		RLSRules:  rulesJSON,
	}
	if err := s.repo.UpsertProjectMember(ctx, member); err != nil {
		return nil, customerror.NewInternalError("failed to assign project member", err)
	}

	saved, err := s.repo.GetProjectMember(ctx, projID, req.UserID)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return memberResponse(member), nil
	}
	return memberResponse(saved), nil
}

func (s *ProjectService) requireProject(ctx context.Context, wsID, projID string) (*entity.Project, error) {
	project, err := s.repo.GetProjectByID(ctx, wsID, projID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load project", err)
	}
	if project == nil {
		return nil, customerror.NewNotFoundError("project not found", nil)
	}
	return project, nil
}

// validateRLSRules rejects rules whose column is not a plain SQL identifier or
// whose operator is outside the supported comparison set. Column names are
// interpolated into SQL text, so this check is what keeps RLS injection-safe.
func validateRLSRules(rules []entity.RLSRuleSpec) error {
	for _, rule := range rules {
		column := strings.TrimSpace(rule.Column)
		if column == "" {
			return customerror.NewBadRequestError("rls rule column is required", nil)
		}
		if !sqlident.IsQualified(column) {
			return customerror.NewBadRequestError(fmt.Sprintf("rls rule column %q is not a valid identifier", column), nil)
		}

		operator := strings.TrimSpace(rule.Operator)
		if operator == "" {
			continue
		}
		if _, ok := rlsOperators[operator]; !ok {
			return customerror.NewBadRequestError(fmt.Sprintf("rls rule operator %q is not supported", operator), nil)
		}

		if rule.Value == nil && strings.TrimSpace(rule.ValueFromUser) == "" {
			return customerror.NewBadRequestError(
				fmt.Sprintf("rls rule for column %q needs either value or value_from_user", column), nil)
		}
	}
	return nil
}

func projectResponse(p *entity.Project) *response.ProjectResponse {
	return &response.ProjectResponse{
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		Name:        p.Name,
		Slug:        p.Slug,
		Description: p.Description,
		Icon:        p.Icon,
		CreatedAt:   p.CreatedAt,
	}
}

func memberResponse(m *entity.ProjectMember) *response.ProjectMemberResponse {
	rules := m.RLSRules
	if len(rules) == 0 {
		rules = json.RawMessage("[]")
	}
	return &response.ProjectMemberResponse{
		ID:           m.ID,
		ProjectID:    m.ProjectID,
		UserID:       m.UserID,
		UserEmail:    m.UserEmail,
		UserFullName: m.UserFullName,
		Role:         m.Role,
		RLSRules:     rules,
		CreatedAt:    m.CreatedAt,
	}
}

// slugify converts a display name into a URL-safe lowercase identifier.
func slugify(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash && b.Len() > 0:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

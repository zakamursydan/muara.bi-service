package service

import (
	"context"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/pkg/customerror"
)

// Scope identifies whose permissions to resolve and against which tenant.
// ProjectID may be empty for workspace-scoped routes.
type Scope struct {
	WorkspaceID string
	ProjectID   string
	UserID      string
	Attributes  map[string]any
}

// Access is the resolved authorization context for a single request.
type Access struct {
	WorkspaceRole string
	ProjectRole   string
	Permissions   entity.PermissionSet
	RLSRules      []duckdb.RLSRule
}

func (a *Access) Can(permission entity.Permission) bool {
	return a.Permissions.Has(permission)
}

type AccessServiceProvider interface {
	Resolve(ctx context.Context, scope Scope) (*Access, error)
	ProjectForDashboard(ctx context.Context, workspaceID, dashboardID string) (string, error)
	ProjectForWidget(ctx context.Context, workspaceID, widgetID string) (string, error)
}

// Consumer-side interfaces keep AccessService testable and decoupled from the
// full pgsql repository surfaces.
type (
	WorkspaceRoleReader interface {
		GetWorkspaceRole(ctx context.Context, workspaceID, userID string) (string, error)
	}
	ProjectAccessReader interface {
		GetProjectByID(ctx context.Context, workspaceID, id string) (*entity.Project, error)
		GetProjectMember(ctx context.Context, projectID, userID string) (*entity.ProjectMember, error)
	}
	DashboardProjectReader interface {
		GetProjectIDByDashboard(ctx context.Context, workspaceID, dashboardID string) (string, error)
	}
	WidgetProjectReader interface {
		GetProjectIDByWidget(ctx context.Context, workspaceID, widgetID string) (string, error)
	}
)

type AccessService struct {
	userRepo      WorkspaceRoleReader
	projectRepo   ProjectAccessReader
	dashboardRepo DashboardProjectReader
	widgetRepo    WidgetProjectReader
}

func NewAccessService(
	userRepo WorkspaceRoleReader,
	projectRepo ProjectAccessReader,
	dashboardRepo DashboardProjectReader,
	widgetRepo WidgetProjectReader,
) *AccessService {
	return &AccessService{
		userRepo:      userRepo,
		projectRepo:   projectRepo,
		dashboardRepo: dashboardRepo,
		widgetRepo:    widgetRepo,
	}
}

// Resolve unions the caller's workspace and project grants and derives the row
// filters that must be applied to every query they run in that project.
//
// Two rules keep this fail-closed:
//   - A user who is not a member of the project loses all permissions unless
//     they administer the whole workspace, so no query can run unfiltered.
//   - Only workspace OWNER/ADMIN are exempt from RLS. Everyone else is
//     restricted by their own project_members.rls_rules.
func (s *AccessService) Resolve(ctx context.Context, scope Scope) (*Access, error) {
	denied := &Access{Permissions: entity.PermissionSet{}}
	if scope.WorkspaceID == "" || scope.UserID == "" {
		return denied, nil
	}

	wsRole, err := s.userRepo.GetWorkspaceRole(ctx, scope.WorkspaceID, scope.UserID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to resolve workspace role", err)
	}
	if wsRole == "" {
		return denied, nil
	}

	access := &Access{
		WorkspaceRole: wsRole,
		Permissions:   entity.PermissionsForWorkspaceRole(wsRole),
	}
	isWorkspaceAdmin := wsRole == entity.RoleOwner || wsRole == entity.RoleAdmin

	if scope.ProjectID == "" {
		return access, nil
	}

	// Refuse to evaluate membership against a project that does not belong to
	// the workspace in the URL. Without this a caller who administers project P
	// in workspace B could reach P's grants through /workspaces/A/projects/P.
	project, err := s.projectRepo.GetProjectByID(ctx, scope.WorkspaceID, scope.ProjectID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to resolve project", err)
	}
	if project == nil {
		access.Permissions = entity.PermissionSet{}
		return access, nil
	}

	member, err := s.projectRepo.GetProjectMember(ctx, scope.ProjectID, scope.UserID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to resolve project membership", err)
	}
	if member == nil {
		if isWorkspaceAdmin {
			return access, nil
		}
		access.Permissions = entity.PermissionSet{}
		return access, nil
	}

	access.ProjectRole = member.Role
	access.Permissions.Merge(entity.PermissionsForProjectRole(member.Role))
	if isWorkspaceAdmin {
		return access, nil
	}

	specs, err := ParseRLSRuleSpecs(member.RLSRules)
	if err != nil {
		return nil, err
	}
	rules, err := ResolveRLSRules(specs, scope.Attributes)
	if err != nil {
		return nil, err
	}
	access.RLSRules = rules

	return access, nil
}

func (s *AccessService) ProjectForDashboard(ctx context.Context, workspaceID, dashboardID string) (string, error) {
	if workspaceID == "" || dashboardID == "" {
		return "", nil
	}
	return s.dashboardRepo.GetProjectIDByDashboard(ctx, workspaceID, dashboardID)
}

func (s *AccessService) ProjectForWidget(ctx context.Context, workspaceID, widgetID string) (string, error) {
	if workspaceID == "" || widgetID == "" {
		return "", nil
	}
	return s.widgetRepo.GetProjectIDByWidget(ctx, workspaceID, widgetID)
}

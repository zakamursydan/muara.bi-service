package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/pkg/customerror"
)

// stubAccessStore satisfies all four AccessService dependencies at once. By
// default it reports that the project exists in the requested workspace.
type stubAccessStore struct {
	wsRole      string
	wsRoleErr   error
	project     *entity.Project
	projectGone bool
	member      *entity.ProjectMember
	memberErr   error
	dashProj    string
	widgetProj  string
}

func (s *stubAccessStore) GetWorkspaceRole(_ context.Context, _, _ string) (string, error) {
	return s.wsRole, s.wsRoleErr
}

func (s *stubAccessStore) GetProjectByID(_ context.Context, _, _ string) (*entity.Project, error) {
	if s.projectGone {
		return nil, nil
	}
	if s.project != nil {
		return s.project, nil
	}
	return &entity.Project{ID: "proj"}, nil
}

func (s *stubAccessStore) GetProjectMember(_ context.Context, _, _ string) (*entity.ProjectMember, error) {
	return s.member, s.memberErr
}

func (s *stubAccessStore) GetProjectIDByDashboard(_ context.Context, _, _ string) (string, error) {
	return s.dashProj, nil
}

func (s *stubAccessStore) GetProjectIDByWidget(_ context.Context, _, _ string) (string, error) {
	return s.widgetProj, nil
}

func newAccessService(store *stubAccessStore) *AccessService {
	return NewAccessService(store, store, store, store)
}

func member(role, rules string) *entity.ProjectMember {
	return &entity.ProjectMember{Role: role, RLSRules: json.RawMessage(rules)}
}

func assertErrorCode(t *testing.T, err error, code customerror.ErrorCode) {
	t.Helper()
	var appErr *customerror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *customerror.AppError, got %v", err)
	}
	if appErr.Code != code {
		t.Fatalf("expected code %s, got %s (%v)", code, appErr.Code, appErr)
	}
}

func TestResolveRLSRules(t *testing.T) {
	specs := []entity.RLSRuleSpec{
		{Column: "branch_id", Operator: "=", ValueFromUser: "branch_id"},
		{Column: "status", Operator: "!=", Value: "ARCHIVED"},
	}

	rules, err := ResolveRLSRules(specs, map[string]any{"branch_id": "SUB_01"})
	if err != nil {
		t.Fatalf("ResolveRLSRules() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Value != "SUB_01" || rules[1].Value != "ARCHIVED" {
		t.Fatalf("unexpected bound values: %+v", rules)
	}
}

func TestResolveRLSRulesFailsClosedOnMissingAttribute(t *testing.T) {
	specs := []entity.RLSRuleSpec{{Column: "branch_id", Operator: "=", ValueFromUser: "branch_id"}}

	_, err := ResolveRLSRules(specs, map[string]any{"team": "sales"})
	assertErrorCode(t, err, customerror.ErrForbidden)

	_, err = ResolveRLSRules(specs, map[string]any{"branch_id": nil})
	assertErrorCode(t, err, customerror.ErrForbidden)
}

func TestResolveRLSRulesRejectsNonScalarAttribute(t *testing.T) {
	specs := []entity.RLSRuleSpec{{Column: "branch_id", Operator: "=", ValueFromUser: "branch_id"}}

	_, err := ResolveRLSRules(specs, map[string]any{"branch_id": []any{"A", "B"}})
	assertErrorCode(t, err, customerror.ErrForbidden)
}

func TestResolveRLSRulesRejectsUnsafeColumn(t *testing.T) {
	specs := []entity.RLSRuleSpec{{Column: "branch_id = 'x' OR 1=1 --", ValueFromUser: "branch_id"}}

	_, err := ResolveRLSRules(specs, map[string]any{"branch_id": "SUB_01"})
	assertErrorCode(t, err, customerror.ErrBadRequest)
}

func TestResolveRLSRulesRejectsRuleWithoutValueSource(t *testing.T) {
	specs := []entity.RLSRuleSpec{{Column: "branch_id", Operator: "="}}

	_, err := ResolveRLSRules(specs, nil)
	assertErrorCode(t, err, customerror.ErrBadRequest)
}

func TestParseRLSRuleSpecsTreatsEmptyAsNoRestriction(t *testing.T) {
	for _, raw := range []string{"", "   ", "null", "[]"} {
		specs, err := ParseRLSRuleSpecs(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("ParseRLSRuleSpecs(%q) error = %v", raw, err)
		}
		if len(specs) != 0 {
			t.Fatalf("ParseRLSRuleSpecs(%q) = %+v, want empty", raw, specs)
		}
	}
}

func TestParseRLSRuleSpecsRejectsMalformedJSON(t *testing.T) {
	_, err := ParseRLSRuleSpecs(json.RawMessage(`[{"column": `))
	assertErrorCode(t, err, customerror.ErrBadRequest)
}

func TestResolveDeniesNonWorkspaceMember(t *testing.T) {
	svc := newAccessService(&stubAccessStore{wsRole: ""})

	access, err := svc.Resolve(context.Background(), Scope{WorkspaceID: "ws", UserID: "u1"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.Permissions) != 0 {
		t.Fatalf("expected no permissions, got %v", access.Permissions)
	}
}

func TestResolveDeniesProjectScopedRouteForNonMember(t *testing.T) {
	// A workspace VIEWER who is not in project_members must not be able to run
	// an unfiltered query against that project.
	svc := newAccessService(&stubAccessStore{wsRole: entity.RoleViewer, member: nil})

	access, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws", ProjectID: "proj", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.Permissions) != 0 {
		t.Fatalf("expected denial for non-member, got %v", access.Permissions)
	}
	if access.Can(entity.PermQueryExecute) {
		t.Fatal("non-member must not be able to execute queries")
	}
}

func TestResolveUnionsWorkspaceAndProjectRoles(t *testing.T) {
	// Workspace VIEWER but project EDITOR: the project grant must elevate.
	svc := newAccessService(&stubAccessStore{
		wsRole: entity.RoleViewer,
		member: member(entity.ProjectRoleEditor, `[]`),
	})

	access, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws", ProjectID: "proj", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !access.Can(entity.PermDashboardWrite) {
		t.Fatal("project EDITOR should be able to write dashboards")
	}
	if access.Can(entity.PermMemberManage) {
		t.Fatal("project EDITOR should not manage members")
	}
	if access.Can(entity.PermConnectionWrite) {
		t.Fatal("project roles must never grant connection authoring")
	}
}

func TestResolveAppliesRLSToNonAdminMembers(t *testing.T) {
	svc := newAccessService(&stubAccessStore{
		wsRole: entity.RoleViewer,
		member: member(entity.ProjectRoleViewer,
			`[{"column":"branch_id","operator":"=","value_from_user":"branch_id"}]`),
	})

	access, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws",
		ProjectID:   "proj",
		UserID:      "u1",
		Attributes:  map[string]any{"branch_id": "SUB_01"},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.RLSRules) != 1 || access.RLSRules[0].Value != "SUB_01" {
		t.Fatalf("expected branch filter for SUB_01, got %+v", access.RLSRules)
	}
}

func TestResolveFailsClosedWhenMemberLacksRequiredAttribute(t *testing.T) {
	svc := newAccessService(&stubAccessStore{
		wsRole: entity.RoleEditor,
		member: member(entity.ProjectRoleEditor,
			`[{"column":"branch_id","operator":"=","value_from_user":"branch_id"}]`),
	})

	_, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws", ProjectID: "proj", UserID: "u1", Attributes: map[string]any{},
	})
	assertErrorCode(t, err, customerror.ErrForbidden)
}

func TestResolveExemptsWorkspaceAdminFromRLS(t *testing.T) {
	svc := newAccessService(&stubAccessStore{
		wsRole: entity.RoleAdmin,
		member: member(entity.ProjectRoleViewer,
			`[{"column":"branch_id","operator":"=","value_from_user":"branch_id"}]`),
	})

	access, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws", ProjectID: "proj", UserID: "u1", Attributes: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.RLSRules) != 0 {
		t.Fatalf("workspace admin should be unfiltered, got %+v", access.RLSRules)
	}
	if !access.Can(entity.PermMemberManage) {
		t.Fatal("workspace admin should hold every permission")
	}
}

func TestResolveWorkspaceScopedRouteIgnoresProjectMembership(t *testing.T) {
	svc := newAccessService(&stubAccessStore{wsRole: entity.RoleEngineer})

	access, err := svc.Resolve(context.Background(), Scope{WorkspaceID: "ws", UserID: "u1"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !access.Can(entity.PermConnectionWrite) {
		t.Fatal("workspace ENGINEER should author connections")
	}
	if access.Can(entity.PermMemberManage) {
		t.Fatal("workspace ENGINEER should not manage members")
	}
}

func TestResolveUnknownRoleFailsClosed(t *testing.T) {
	svc := newAccessService(&stubAccessStore{wsRole: "SUPERUSER"})

	access, err := svc.Resolve(context.Background(), Scope{WorkspaceID: "ws", UserID: "u1"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.Permissions) != 0 {
		t.Fatalf("unknown role must grant nothing, got %v", access.Permissions)
	}
}

func TestResolveDeniesProjectFromAnotherWorkspace(t *testing.T) {
	// The caller administers this project, but it lives in a different
	// workspace than the one in the URL, so no grant may be evaluated.
	svc := newAccessService(&stubAccessStore{
		wsRole:      entity.RoleViewer,
		projectGone: true,
		member:      member(entity.ProjectRoleAdmin, `[]`),
	})

	access, err := svc.Resolve(context.Background(), Scope{
		WorkspaceID: "ws", ProjectID: "foreign-proj", UserID: "u1",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(access.Permissions) != 0 {
		t.Fatalf("expected denial for a foreign project, got %v", access.Permissions)
	}
}

func TestPermissionsForRoleReturnsIndependentCopy(t *testing.T) {
	set := entity.PermissionsForWorkspaceRole(entity.RoleViewer)
	set.Add(entity.PermMemberManage)

	fresh := entity.PermissionsForWorkspaceRole(entity.RoleViewer)
	if fresh.Has(entity.PermMemberManage) {
		t.Fatal("mutating a returned permission set must not affect the role matrix")
	}
}

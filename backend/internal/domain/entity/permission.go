package entity

// Permission is a single authorizable action inside a workspace or project.
type Permission string

const (
	PermConnectionRead  Permission = "connection:read"
	PermConnectionWrite Permission = "connection:write"
	PermSemanticRead    Permission = "semantic:read"
	PermSemanticWrite   Permission = "semantic:write"
	PermProjectRead     Permission = "project:read"
	PermProjectWrite    Permission = "project:write"
	PermDashboardRead   Permission = "dashboard:read"
	PermDashboardWrite  Permission = "dashboard:write"
	PermQueryExecute    Permission = "query:execute"
	PermMemberManage    Permission = "member:manage"
	PermSharePublish    Permission = "share:publish"
)

// PermissionSet is the union of everything a caller may do in one scope.
type PermissionSet map[Permission]struct{}

func NewPermissionSet(permissions ...Permission) PermissionSet {
	set := make(PermissionSet, len(permissions))
	set.Add(permissions...)
	return set
}

func (s PermissionSet) Has(p Permission) bool {
	_, ok := s[p]
	return ok
}

func (s PermissionSet) Add(permissions ...Permission) {
	for _, p := range permissions {
		s[p] = struct{}{}
	}
}

// Merge folds other into s. It mutates s, so callers must pass a set they own.
func (s PermissionSet) Merge(other PermissionSet) {
	for p := range other {
		s[p] = struct{}{}
	}
}

func (s PermissionSet) clone() PermissionSet {
	out := make(PermissionSet, len(s))
	out.Merge(s)
	return out
}

// readerPermissions are shared by every role that can open a dashboard and run
// its widgets, which is what interactive filters and exports need.
func readerPermissions() PermissionSet {
	return NewPermissionSet(
		PermProjectRead,
		PermDashboardRead,
		PermQueryExecute,
	)
}

// builderPermissions add authoring of analytics content on top of reading.
func builderPermissions(extra ...Permission) PermissionSet {
	set := readerPermissions()
	set.Add(
		PermConnectionRead,
		PermSemanticRead,
		PermSemanticWrite,
		PermDashboardWrite,
	)
	set.Add(extra...)
	return set
}

var allPermissions = NewPermissionSet(
	PermConnectionRead, PermConnectionWrite,
	PermSemanticRead, PermSemanticWrite,
	PermProjectRead, PermProjectWrite,
	PermDashboardRead, PermDashboardWrite,
	PermQueryExecute, PermMemberManage, PermSharePublish,
)

// workspaceRolePermissions maps workspace_members.role to its grant set.
// ENGINEER is EDITOR plus connection authoring; VIEWER is read-only.
var workspaceRolePermissions = map[string]PermissionSet{
	RoleOwner:    allPermissions,
	RoleAdmin:    allPermissions,
	RoleEngineer: builderPermissions(PermProjectWrite, PermConnectionWrite),
	RoleEditor:   builderPermissions(PermProjectWrite),
	RoleViewer:   readerPermissions(),
}

// projectRolePermissions maps project_members.role to its grant set. Project
// roles never grant connection authoring, which stays a workspace concern.
var projectRolePermissions = map[string]PermissionSet{
	ProjectRoleAdmin:  builderPermissions(PermProjectWrite, PermMemberManage, PermSharePublish),
	ProjectRoleEditor: builderPermissions(),
	ProjectRoleViewer: readerPermissions(),
}

// PermissionsForWorkspaceRole returns a fresh set owned by the caller. An
// unknown or empty role yields no permissions, so lookups fail closed.
func PermissionsForWorkspaceRole(role string) PermissionSet {
	return workspaceRolePermissions[role].clone()
}

// PermissionsForProjectRole returns a fresh set owned by the caller.
func PermissionsForProjectRole(role string) PermissionSet {
	return projectRolePermissions[role].clone()
}

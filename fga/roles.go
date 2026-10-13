package fga

// Project-level roles assignable at project scope. Match the FGA model.
const (
	RoleProjectManager = "project_manager"
	RoleSurveyManager  = "survey_manager"
	RoleGISAnalyst     = "gis_analyst"
	RoleMiningEngineer = "mining_engineer"
	RoleReviewer       = "reviewer"
	RoleClientAdmin    = "client_admin"
	RoleClientViewer   = "client_viewer"
	RoleAPIUser        = "api_user"
)

// Organisation-level roles.
const (
	RoleSuperAdmin = "super_admin"
	RoleOrgAdmin   = "org_admin"
	RoleMember     = "member"
)

// RoleWorkspaceAdmin manages a workspace and its access, and is project
// manager on every project in it. The other workspace roles are the project
// roles below project manager, inherited by each project in the workspace.
const RoleWorkspaceAdmin = "admin"

// Team roles. A maintainer is also a member.
const (
	RoleTeamMaintainer = "maintainer"
	RoleTeamMember     = "member"
)

// ValidProjectRoles is the set of FGA relations assignable at project scope.
var ValidProjectRoles = []string{
	RoleProjectManager, RoleSurveyManager, RoleGISAnalyst, RoleMiningEngineer,
	RoleReviewer, RoleClientAdmin, RoleClientViewer, RoleAPIUser,
}

// ValidOrgRoles is the set of FGA relations assignable at organisation scope.
var ValidOrgRoles = []string{RoleSuperAdmin, RoleOrgAdmin, RoleMember}

// ValidWorkspaceRoles is the set of FGA relations assignable at workspace scope.
var ValidWorkspaceRoles = []string{
	RoleWorkspaceAdmin, RoleSurveyManager, RoleGISAnalyst, RoleMiningEngineer,
	RoleReviewer, RoleClientAdmin, RoleClientViewer, RoleAPIUser,
}

// ValidTeamRoles is the set of FGA relations assignable at team scope.
var ValidTeamRoles = []string{RoleTeamMaintainer, RoleTeamMember}

// ClientRoles is the subset of project roles a client_admin may assign to
// other client users (enforces SCRUM-114 footnote 5).
var ClientRoles = []string{RoleClientAdmin, RoleClientViewer}

// Computed permissions on a project (match the FGA authorization model).
const (
	PermCanUpload            = "can_upload"
	PermCanApprove           = "can_approve"
	PermCanCreateMeasurement = "can_create_measurement"
	PermCanRunAnalysis       = "can_run_analysis"
	PermCanExport            = "can_export"
	PermCanViewDraft         = "can_view_draft"
	PermCanViewPublished     = "can_view_published"
	PermCanManageUsers       = "can_manage_users"
	PermCanCallAPI           = "can_call_api"
)

// All9ProjectPermissions is the complete set used by /auth/permissions BatchCheck.
var All9ProjectPermissions = []string{
	PermCanUpload, PermCanApprove, PermCanCreateMeasurement, PermCanRunAnalysis,
	PermCanExport, PermCanViewDraft, PermCanViewPublished, PermCanManageUsers, PermCanCallAPI,
}

// Computed permissions on a workspace and on a team.
const (
	PermWorkspaceCanView          = "can_view"
	PermWorkspaceCanManage        = "can_manage"
	PermWorkspaceCanCreateProject = "can_create_project"
	PermTeamCanManage             = "can_manage"
)

// Computed permissions on a survey. Survey-scoped relations are NOT the same
// as project-scoped — survey's `can_view` is derived from `can_view_published`
// on the parent project (see infra/fga/model.fga), so passing PermCanViewPublished
// to a survey-typed Check would produce an OpenFGA validation_error.
// Use the survey-scoped constants below when the FGA object type is `survey`.
const (
	PermSurveyCanView      = "can_view"       // ≡ can_view_published from project
	PermSurveyCanViewDraft = "can_view_draft" // ≡ can_view_draft from project
	PermSurveyCanApprove   = "can_approve"    // ≡ can_approve from project
)

// DefaultClientCreatorRole is the org-scope role assigned to the user who
// creates a new client/organisation. Per SCRUM-117 — configurable.
const DefaultClientCreatorRole = RoleSuperAdmin

// IsValidProjectRole returns true if r is in ValidProjectRoles.
func IsValidProjectRole(r string) bool {
	for _, v := range ValidProjectRoles {
		if v == r {
			return true
		}
	}
	return false
}

// IsValidOrgRole returns true if r is in ValidOrgRoles.
func IsValidOrgRole(r string) bool {
	for _, v := range ValidOrgRoles {
		if v == r {
			return true
		}
	}
	return false
}

// IsValidWorkspaceRole returns true if r is in ValidWorkspaceRoles.
func IsValidWorkspaceRole(r string) bool {
	for _, v := range ValidWorkspaceRoles {
		if v == r {
			return true
		}
	}
	return false
}

// IsValidTeamRole returns true if r is in ValidTeamRoles.
func IsValidTeamRole(r string) bool {
	for _, v := range ValidTeamRoles {
		if v == r {
			return true
		}
	}
	return false
}

// IsClientRole reports whether the role is one of the client-side project roles.
func IsClientRole(r string) bool {
	for _, v := range ClientRoles {
		if v == r {
			return true
		}
	}
	return false
}

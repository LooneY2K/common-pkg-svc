package fga

// Object type constants — match the FGA authorization model exactly.
const (
	TypeUser         = "user"
	TypeOrganisation = "organisation"
	TypeProject      = "project"
	TypeSurvey       = "survey"
	TypeWorkspace    = "workspace"
)

// Subject formats a generic FGA reference: "<type>:<id>".
// Prefer the typed helpers below in application code.
func Subject(typ, id string) string {
	return typ + ":" + id
}

// UserSubject formats the FGA tuple subject for a user.
//
// CANONICAL ID: register_users.id (matches the JWT user_id claim issued by
// AuthMiddleware). Per SCRUM-120: always use this helper — never concatenate
// "user:" directly in tuple writes (grep-friendly invariant for code review).
func UserSubject(registerUserID string) string { return Subject(TypeUser, registerUserID) }
func OrganisationSubject(id string) string     { return Subject(TypeOrganisation, id) }
func ProjectSubject(id string) string          { return Subject(TypeProject, id) }
func SurveySubject(id string) string           { return Subject(TypeSurvey, id) }
func WorkspaceSubject(id string) string        { return Subject(TypeWorkspace, id) }

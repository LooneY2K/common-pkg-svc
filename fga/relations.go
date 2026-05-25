package fga

// Parent-child relation names used in link tuples between object types.
// Example: {user: ProjectSubject(X), relation: RelationProject, object: SurveySubject(Y)}
// means "survey Y belongs to project X" — lets FGA derive can_view on the
// survey via the project's permission graph.
const (
	RelationOrganisation = "organisation" // organisation→project link
	RelationProject      = "project"      // project→survey, project→workspace links
)

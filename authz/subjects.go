package authz

import "strings"

// Object types in the model. A file and a folder are both `node` — the rules
// are identical and one type lets `parent` point at either.
const (
	TypeUser = "user"
	TypeNode = "node"
	TypeLink = "link"
)

// Directly assignable relations — what a share writes.
const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// RelationParent links a child node to its folder. The permission relations
// below inherit through it, so granting on a folder reaches everything in it.
const RelationParent = "parent"

// Effective permissions — what handlers check. These are computed, never
// written: passing one to Share is a programming error and is rejected.
const (
	PermCanView   = "can_view"
	PermCanEdit   = "can_edit"
	PermCanManage = "can_manage"
)

// AssignableRoles are the relations Share accepts, weakest first.
var AssignableRoles = []string{RoleViewer, RoleEditor, RoleOwner}

// UserSubject formats the tuple subject for a person.
//
// CANONICAL ID: users._id from MongoDB, which is also the JWT user_id claim.
// Always go through this helper — a hand-written "user:"+id is how the two
// drift apart.
func UserSubject(userID string) string { return TypeUser + ":" + userID }

// NodeObject formats the tuple object for a file or folder.
func NodeObject(nodeID string) string { return TypeNode + ":" + nodeID }

// LinkSubject formats the tuple subject for a share link. Revoking a link is
// deleting its tuple; expiry is the application's job, because a tuple has no
// clock.
func LinkSubject(linkID string) string { return TypeLink + ":" + linkID }

// Everyone is the wildcard subject: a tuple with this subject makes a node
// readable by any authenticated user. Only `viewer` accepts it, so there is
// no way to grant the world write access by mistake.
const Everyone = TypeUser + ":*"

// stripNode removes the type prefix that OpenFGA returns on ListObjects, so
// callers get something they can hand straight to a Mongo query. This is the
// opposite of the fga package's convention, which preserves the prefix.
func stripNode(object string) string {
	return strings.TrimPrefix(object, TypeNode+":")
}

// IsAssignableRole reports whether r can be written as a direct grant.
func IsAssignableRole(r string) bool {
	for _, v := range AssignableRoles {
		if v == r {
			return true
		}
	}
	return false
}

// IsPermission reports whether r is a computed permission that can be checked.
func IsPermission(r string) bool {
	switch r {
	case PermCanView, PermCanEdit, PermCanManage:
		return true
	}
	return false
}

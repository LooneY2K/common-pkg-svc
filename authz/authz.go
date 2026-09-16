// Package authz is the Puckoops Drive authorization model and the functions
// that answer questions about it.
//
// The model is a single `node` type — a file and a folder are the same thing —
// with three assignable roles (owner, editor, viewer) and three computed
// permissions (can_manage, can_edit, can_view) that inherit down the `parent`
// chain. Granting viewer on a folder therefore reaches every file inside it,
// at any depth.
//
// Handlers should ask for a permission, never for a role:
//
//	ok, err := az.CanEdit(ctx, userID, nodeID)
//
// Checking `editor` directly would miss the owner of the file and everyone who
// inherited access from a parent folder — which is the whole point of the
// model. IsPermission exists so that mistake fails loudly instead of quietly
// returning false.
//
// Package fga in this module is the other product's model (organisation /
// project / survey) and is unrelated; only its transport wrapper is shared.
package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LooneY2K/common-pkg-svc/fga"
	openfga "github.com/openfga/go-sdk"
)

// Authorizer answers relationship questions about nodes. Safe for concurrent
// use; it holds no state beyond the client.
type Authorizer struct {
	client fga.Client
}

// New wraps an fga.Client. Use fga.NewClientNoAuth for a local OpenFGA
// started without a preshared key, or fga.NewClient in deployed environments.
func New(client fga.Client) (*Authorizer, error) {
	if client == nil {
		return nil, errors.New("authz: client is required")
	}
	return &Authorizer{client: client}, nil
}

/* --------------------------------------------------------------- checks -- */

// Can reports whether a user holds a computed permission on a node. The four
// helpers below are the ones to reach for; this is the escape hatch when the
// permission is itself a variable.
func (a *Authorizer) Can(ctx context.Context, userID, permission, nodeID string) (bool, error) {
	if !IsPermission(permission) {
		// An assignable role would silently answer the wrong question: a file's
		// owner has no `editor` tuple, so Check("editor") on their own file is
		// false.
		return false, fmt.Errorf("authz: %q is not a checkable permission (use can_view, can_edit or can_manage)", permission)
	}
	if userID == "" || nodeID == "" {
		return false, errors.New("authz: userID and nodeID are required")
	}
	return a.client.Check(ctx, UserSubject(userID), permission, NodeObject(nodeID))
}

// CanView reports read access, whether granted directly, inherited from a
// parent folder, implied by edit access, or public.
func (a *Authorizer) CanView(ctx context.Context, userID, nodeID string) (bool, error) {
	return a.Can(ctx, userID, PermCanView, nodeID)
}

// CanEdit reports write access: rename, move, overwrite, upload into.
func (a *Authorizer) CanEdit(ctx context.Context, userID, nodeID string) (bool, error) {
	return a.Can(ctx, userID, PermCanEdit, nodeID)
}

// CanManage reports ownership-level access: sharing, revoking, deleting.
func (a *Authorizer) CanManage(ctx context.Context, userID, nodeID string) (bool, error) {
	return a.Can(ctx, userID, PermCanManage, nodeID)
}

// CanViaLink reports whether a share link grants a permission on a node. The
// link id is the credential, so the caller must have already resolved it from
// the request and checked its expiry — a tuple has no clock.
func (a *Authorizer) CanViaLink(ctx context.Context, linkID, permission, nodeID string) (bool, error) {
	if !IsPermission(permission) {
		return false, fmt.Errorf("authz: %q is not a checkable permission", permission)
	}
	if linkID == "" || nodeID == "" {
		return false, errors.New("authz: linkID and nodeID are required")
	}
	return a.client.Check(ctx, LinkSubject(linkID), permission, NodeObject(nodeID))
}

// Permissions is the full answer for one node, which is what a UI needs: the
// difference between "can open" and "can open and rename" decides which
// buttons render.
type Permissions struct {
	CanView   bool `json:"can_view"`
	CanEdit   bool `json:"can_edit"`
	CanManage bool `json:"can_manage"`
}

// Permissions resolves all three permissions in a single round trip. Prefer
// it over three CanX calls whenever the caller needs more than one.
func (a *Authorizer) Permissions(ctx context.Context, userID, nodeID string) (Permissions, error) {
	if userID == "" || nodeID == "" {
		return Permissions{}, errors.New("authz: userID and nodeID are required")
	}

	object := NodeObject(nodeID)
	results, err := a.client.BatchCheck(ctx, UserSubject(userID), []fga.CheckRequest{
		{Relation: PermCanView, Object: object},
		{Relation: PermCanEdit, Object: object},
		{Relation: PermCanManage, Object: object},
	})
	if err != nil {
		return Permissions{}, err
	}

	return Permissions{
		CanView:   results[PermCanView+":"+object],
		CanEdit:   results[PermCanEdit+":"+object],
		CanManage: results[PermCanManage+":"+object],
	}, nil
}

// PermissionsForNodes resolves every permission for a whole listing in one
// round trip, keyed by node id. This is the call a folder view wants: the
// alternative is three checks per row, which is where a file list starts
// taking a second to paint.
//
// Nodes the user cannot see at all still appear in the map, with every field
// false. Duplicate ids are collapsed.
func (a *Authorizer) PermissionsForNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]Permissions, error) {
	if userID == "" {
		return nil, errors.New("authz: userID is required")
	}

	out := make(map[string]Permissions, len(nodeIDs))
	checks := make([]fga.CheckRequest, 0, len(nodeIDs)*3)
	for _, id := range nodeIDs {
		if id == "" {
			continue
		}
		if _, seen := out[id]; seen {
			continue
		}
		out[id] = Permissions{}
		object := NodeObject(id)
		checks = append(checks,
			fga.CheckRequest{Relation: PermCanView, Object: object},
			fga.CheckRequest{Relation: PermCanEdit, Object: object},
			fga.CheckRequest{Relation: PermCanManage, Object: object},
		)
	}
	if len(checks) == 0 {
		return out, nil
	}

	results, err := a.client.BatchCheck(ctx, UserSubject(userID), checks)
	if err != nil {
		return nil, err
	}

	for id := range out {
		object := NodeObject(id)
		out[id] = Permissions{
			CanView:   results[PermCanView+":"+object],
			CanEdit:   results[PermCanEdit+":"+object],
			CanManage: results[PermCanManage+":"+object],
		}
	}
	return out, nil
}

/* ---------------------------------------------------------------- lists -- */

// ListViewable returns the ids of every node the user can read, including the
// ones reached through a shared parent folder. Ids come back bare, ready for a
// Mongo query.
func (a *Authorizer) ListViewable(ctx context.Context, userID string) ([]string, error) {
	return a.list(ctx, userID, PermCanView)
}

// ListEditable returns the ids of every node the user can write to.
func (a *Authorizer) ListEditable(ctx context.Context, userID string) ([]string, error) {
	return a.list(ctx, userID, PermCanEdit)
}

// ListManageable returns the ids of every node the user owns or manages.
func (a *Authorizer) ListManageable(ctx context.Context, userID string) ([]string, error) {
	return a.list(ctx, userID, PermCanManage)
}

func (a *Authorizer) list(ctx context.Context, userID, permission string) ([]string, error) {
	if userID == "" {
		return nil, errors.New("authz: userID is required")
	}
	objects, err := a.client.ListObjects(ctx, UserSubject(userID), permission, TypeNode)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(objects))
	for _, o := range objects {
		ids = append(ids, stripNode(o))
	}
	return ids, nil
}

/* --------------------------------------------------------------- grants -- */

// Grant is one direct relationship on a node: who, and as what.
type Grant struct {
	// Subject is the raw tuple subject: "user:<id>", "link:<id>" or "user:*".
	Subject string `json:"subject"`
	// UserID is set when Subject names a person, empty otherwise.
	UserID string `json:"user_id,omitempty"`
	// LinkID is set when Subject names a share link, empty otherwise.
	LinkID string `json:"link_id,omitempty"`
	// Public is true for the everyone wildcard.
	Public bool `json:"public,omitempty"`
	// Role is owner, editor or viewer.
	Role string `json:"role"`
}

// Share grants a role to a user on a node. Idempotent: re-granting a role the
// user already holds succeeds, because a share dialog that errors on a
// double-click is worse than one that does nothing.
//
// A fresh grant is one round trip. A duplicate costs about a second, because
// OpenFGA answers it with the same error code it uses for invalid input and
// the SDK retries that with backoff before the tuple is read back to tell the
// two apart. Call Replace rather than Share in a loop.
//
// Note this writes a *role*, not a permission — and roles do not stack. A user
// who is already an editor and is then granted viewer holds both tuples and
// keeps edit access; call Unshare for the old role, or Replace, to demote.
func (a *Authorizer) Share(ctx context.Context, nodeID, userID, role string) error {
	if !IsAssignableRole(role) {
		return fmt.Errorf("authz: %q is not an assignable role (use owner, editor or viewer)", role)
	}
	if nodeID == "" || userID == "" {
		return errors.New("authz: nodeID and userID are required")
	}
	return a.write(ctx, UserSubject(userID), role, NodeObject(nodeID))
}

// Unshare removes a role from a user on a node. Idempotent: removing a role
// they never held succeeds.
func (a *Authorizer) Unshare(ctx context.Context, nodeID, userID, role string) error {
	if !IsAssignableRole(role) {
		return fmt.Errorf("authz: %q is not an assignable role", role)
	}
	if nodeID == "" || userID == "" {
		return errors.New("authz: nodeID and userID are required")
	}
	return a.delete(ctx, UserSubject(userID), role, NodeObject(nodeID))
}

// Replace sets a user's role on a node to exactly one value, clearing the
// others. This is what a share dialog's role dropdown should call: Share alone
// would leave a demoted editor still holding edit access.
//
// The grant is written before the old roles are removed, so a failure part way
// through leaves too much access rather than locking the user out.
//
// The current roles are read first and only the differences are written.
// Deleting a tuple that does not exist is an error OpenFGA's SDK retries with
// backoff before giving up, which cost this call about a second per role it
// did not need to touch.
func (a *Authorizer) Replace(ctx context.Context, nodeID, userID, role string) error {
	if !IsAssignableRole(role) {
		return fmt.Errorf("authz: %q is not an assignable role", role)
	}
	if nodeID == "" || userID == "" {
		return errors.New("authz: nodeID and userID are required")
	}

	held, err := a.rolesHeld(ctx, nodeID, userID)
	if err != nil {
		return err
	}

	if !held[role] {
		if err := a.Share(ctx, nodeID, userID, role); err != nil {
			return err
		}
	}
	for _, other := range AssignableRoles {
		if other == role || !held[other] {
			continue
		}
		if err := a.Unshare(ctx, nodeID, userID, other); err != nil {
			return err
		}
	}
	return nil
}

// rolesHeld returns the direct roles a user holds on a node.
func (a *Authorizer) rolesHeld(ctx context.Context, nodeID, userID string) (map[string]bool, error) {
	tuples, err := a.client.Read(ctx, fga.ReadFilter{
		User:   UserSubject(userID),
		Object: NodeObject(nodeID),
	})
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool, len(tuples))
	for _, t := range tuples {
		held[t.Relation] = true
	}
	return held, nil
}

// RevokeUser removes every direct role a user holds on a node. Access
// inherited from a parent folder survives — revoke there, or move the node.
func (a *Authorizer) RevokeUser(ctx context.Context, nodeID, userID string) error {
	if nodeID == "" || userID == "" {
		return errors.New("authz: nodeID and userID are required")
	}

	held, err := a.rolesHeld(ctx, nodeID, userID)
	if err != nil {
		return err
	}
	for _, role := range AssignableRoles {
		if !held[role] {
			continue
		}
		if err := a.Unshare(ctx, nodeID, userID, role); err != nil {
			return err
		}
	}
	return nil
}

// ShareWithEveryone makes a node readable by any authenticated user. Only
// viewer accepts the wildcard, so this cannot grant write access.
func (a *Authorizer) ShareWithEveryone(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return errors.New("authz: nodeID is required")
	}
	return a.write(ctx, Everyone, RoleViewer, NodeObject(nodeID))
}

// UnshareWithEveryone withdraws public read access. Idempotent.
func (a *Authorizer) UnshareWithEveryone(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return errors.New("authz: nodeID is required")
	}
	return a.delete(ctx, Everyone, RoleViewer, NodeObject(nodeID))
}

// ShareLink points a share link at a node with either viewer or editor
// access. Owner is not available to links: a link cannot be allowed to
// reshare or delete.
func (a *Authorizer) ShareLink(ctx context.Context, nodeID, linkID, role string) error {
	if role != RoleViewer && role != RoleEditor {
		return fmt.Errorf("authz: a link can only be viewer or editor, not %q", role)
	}
	if nodeID == "" || linkID == "" {
		return errors.New("authz: nodeID and linkID are required")
	}
	return a.write(ctx, LinkSubject(linkID), role, NodeObject(nodeID))
}

// RevokeLink withdraws a share link's access to a node. Idempotent.
func (a *Authorizer) RevokeLink(ctx context.Context, nodeID, linkID, role string) error {
	if nodeID == "" || linkID == "" {
		return errors.New("authz: nodeID and linkID are required")
	}
	return a.delete(ctx, LinkSubject(linkID), role, NodeObject(nodeID))
}

// Grants lists the direct relationships on a node — everyone the share dialog
// should show. Inherited access is deliberately absent: it belongs to the
// parent, and listing it here would offer a revoke button that cannot work.
func (a *Authorizer) Grants(ctx context.Context, nodeID string) ([]Grant, error) {
	if nodeID == "" {
		return nil, errors.New("authz: nodeID is required")
	}

	tuples, err := a.client.Read(ctx, fga.ReadFilter{Object: NodeObject(nodeID)})
	if err != nil {
		return nil, err
	}

	grants := make([]Grant, 0, len(tuples))
	for _, t := range tuples {
		if !IsAssignableRole(t.Relation) {
			continue // the parent link, and any relation added to the model later
		}
		g := Grant{Subject: t.User, Role: t.Relation}
		switch {
		case t.User == Everyone:
			g.Public = true
		case strings.HasPrefix(t.User, TypeUser+":"):
			g.UserID = strings.TrimPrefix(t.User, TypeUser+":")
		case strings.HasPrefix(t.User, TypeLink+":"):
			g.LinkID = strings.TrimPrefix(t.User, TypeLink+":")
		}
		grants = append(grants, g)
	}
	return grants, nil
}

/* ----------------------------------------------------------------- tree -- */

// CreateNode records a new file or folder: its owner, and its parent when it
// is not at the root. Both tuples go in one transactional write, so a node
// never exists with a parent but no owner.
//
// Call this from the same code path that inserts the document, and treat a
// failure as a failed create — a node with no owner tuple is invisible to
// everyone, including the person who just uploaded it.
func (a *Authorizer) CreateNode(ctx context.Context, nodeID, ownerID, parentID string) error {
	if nodeID == "" || ownerID == "" {
		return errors.New("authz: nodeID and ownerID are required")
	}

	object := NodeObject(nodeID)
	tuples := []fga.TupleRequest{{User: UserSubject(ownerID), Relation: RoleOwner, Object: object}}
	if parentID != "" {
		tuples = append(tuples, fga.TupleRequest{
			User:     NodeObject(parentID),
			Relation: RelationParent,
			Object:   object,
		})
	}

	if err := a.client.WriteTuples(ctx, tuples); err != nil {
		if a.allTuplesPresent(ctx, tuples) {
			return nil
		}
		return err
	}
	return nil
}

// SetParent moves a node under a folder, or to the root when parentID is
// empty. The old parent link is removed after the new one is written, so a
// failure leaves the node reachable from both rather than from neither.
func (a *Authorizer) SetParent(ctx context.Context, nodeID, parentID string) error {
	if nodeID == "" {
		return errors.New("authz: nodeID is required")
	}
	if parentID == nodeID {
		// FGA would accept this and then loop while resolving can_view.
		return errors.New("authz: a node cannot be its own parent")
	}

	object := NodeObject(nodeID)
	existing, err := a.client.Read(ctx, fga.ReadFilter{Relation: RelationParent, Object: object})
	if err != nil {
		return err
	}

	alreadyLinked := false
	for _, t := range existing {
		if t.User == NodeObject(parentID) {
			alreadyLinked = true
			break
		}
	}

	if parentID != "" && !alreadyLinked {
		if err := a.write(ctx, NodeObject(parentID), RelationParent, object); err != nil {
			return err
		}
	}

	for _, t := range existing {
		if parentID != "" && t.User == NodeObject(parentID) {
			continue
		}
		if err := a.delete(ctx, t.User, RelationParent, object); err != nil {
			return err
		}
	}
	return nil
}

// Parent returns the id of the node's parent folder, or "" when it sits at
// the root.
func (a *Authorizer) Parent(ctx context.Context, nodeID string) (string, error) {
	if nodeID == "" {
		return "", errors.New("authz: nodeID is required")
	}
	tuples, err := a.client.Read(ctx, fga.ReadFilter{Relation: RelationParent, Object: NodeObject(nodeID)})
	if err != nil {
		return "", err
	}
	if len(tuples) == 0 {
		return "", nil
	}
	return stripNode(tuples[0].User), nil
}

// DeleteNode removes every tuple pointing at a node: its grants and its
// parent link. Children are left alone — their parent tuples now reference a
// node that no longer exists, which resolves as no inherited access, so
// recurse from the caller if the subtree is going too.
func (a *Authorizer) DeleteNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return errors.New("authz: nodeID is required")
	}
	object := NodeObject(nodeID)
	tuples, err := a.client.Read(ctx, fga.ReadFilter{Object: object})
	if err != nil {
		return err
	}
	for _, t := range tuples {
		if err := a.delete(ctx, t.User, t.Relation, object); err != nil {
			return err
		}
	}
	return nil
}

// Children returns the ids of the nodes whose parent is nodeID. Useful for
// recursing a delete; the document store is the faster answer for listing a
// folder.
func (a *Authorizer) Children(ctx context.Context, nodeID string) ([]string, error) {
	if nodeID == "" {
		return nil, errors.New("authz: nodeID is required")
	}
	// The object is pinned to the bare type: OpenFGA's Read rejects a filter
	// with no object at all, even when the user and relation are given.
	tuples, err := a.client.Read(ctx, fga.ReadFilter{
		User:     NodeObject(nodeID),
		Relation: RelationParent,
		Object:   TypeNode + ":",
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(tuples))
	for _, t := range tuples {
		ids = append(ids, stripNode(t.Object))
	}
	return ids, nil
}

/* ------------------------------------------------------------ internals -- */

// write is an idempotent WriteTuple. OpenFGA reports a duplicate write with
// the same error code it uses for genuinely invalid input, so rather than
// matching on that code the tuple is read back: a write that failed because
// the tuple is already there is a success, anything else is not.
func (a *Authorizer) write(ctx context.Context, user, relation, object string) error {
	err := a.client.WriteTuple(ctx, user, relation, object)
	if err == nil {
		return nil
	}
	if !isInvalidInput(err) {
		return err
	}
	if a.tuplePresent(ctx, user, relation, object) {
		return nil
	}
	return err
}

// delete is an idempotent DeleteTuple, by the same argument as write.
func (a *Authorizer) delete(ctx context.Context, user, relation, object string) error {
	err := a.client.DeleteTuple(ctx, user, relation, object)
	if err == nil {
		return nil
	}
	if !isInvalidInput(err) {
		return err
	}
	if !a.tuplePresent(ctx, user, relation, object) {
		return nil
	}
	return err
}

func (a *Authorizer) tuplePresent(ctx context.Context, user, relation, object string) bool {
	found, err := a.client.Read(ctx, fga.ReadFilter{User: user, Relation: relation, Object: object})
	if err != nil {
		// Unknowable, so assume the write really did fail and let the caller
		// see the original error.
		return false
	}
	return len(found) > 0
}

func (a *Authorizer) allTuplesPresent(ctx context.Context, tuples []fga.TupleRequest) bool {
	if len(tuples) == 0 {
		return false
	}
	for _, t := range tuples {
		if !a.tuplePresent(ctx, t.User, t.Relation, t.Object) {
			return false
		}
	}
	return true
}

// isInvalidInput reports whether the error is OpenFGA's
// write_failed_due_to_invalid_input, which covers writing a tuple that exists
// and deleting one that does not, as well as real validation failures.
func isInvalidInput(err error) bool {
	var validation openfga.FgaApiValidationError
	if errors.As(err, &validation) {
		return validation.ResponseCode() == openfga.ERRORCODE_WRITE_FAILED_DUE_TO_INVALID_INPUT
	}
	var ptr *openfga.FgaApiValidationError
	if errors.As(err, &ptr) && ptr != nil {
		return ptr.ResponseCode() == openfga.ERRORCODE_WRITE_FAILED_DUE_TO_INVALID_INPUT
	}
	return false
}

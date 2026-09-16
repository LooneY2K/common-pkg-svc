package authz

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/LooneY2K/common-pkg-svc/fga"
)

// fakeClient records what reached the transport and replays canned answers.
// It deliberately does not evaluate the model — inheritance is covered by the
// integration test against a real server.
type fakeClient struct {
	writes  []fga.TupleRequest
	deletes []fga.TupleRequest
	checks  []fga.CheckRequest
	reads   []fga.ReadFilter

	checkUser  string
	allow      map[string]bool
	tuples     []fga.TupleRecord
	objects    []string
	writeErr   error
	deleteErr  error
	readErr    error
	listCalled struct {
		user, relation, objectType string
	}
}

func (f *fakeClient) WriteTuple(ctx context.Context, user, relation, object string) error {
	return f.WriteTuples(ctx, []fga.TupleRequest{{User: user, Relation: relation, Object: object}})
}

func (f *fakeClient) WriteTuples(_ context.Context, tuples []fga.TupleRequest) error {
	f.writes = append(f.writes, tuples...)
	return f.writeErr
}

func (f *fakeClient) DeleteTuple(_ context.Context, user, relation, object string) error {
	f.deletes = append(f.deletes, fga.TupleRequest{User: user, Relation: relation, Object: object})
	return f.deleteErr
}

func (f *fakeClient) Check(_ context.Context, user, relation, object string) (bool, error) {
	f.checkUser = user
	f.checks = append(f.checks, fga.CheckRequest{Relation: relation, Object: object})
	return f.allow[relation+":"+object], nil
}

func (f *fakeClient) BatchCheck(_ context.Context, user string, checks []fga.CheckRequest) (map[string]bool, error) {
	f.checkUser = user
	f.checks = append(f.checks, checks...)
	out := make(map[string]bool, len(checks))
	for _, c := range checks {
		key := c.Relation + ":" + c.Object
		out[key] = f.allow[key]
	}
	return out, nil
}

func (f *fakeClient) ListObjects(_ context.Context, user, relation, objectType string) ([]string, error) {
	f.listCalled.user = user
	f.listCalled.relation = relation
	f.listCalled.objectType = objectType
	return f.objects, nil
}

func (f *fakeClient) Read(_ context.Context, filter fga.ReadFilter) ([]fga.TupleRecord, error) {
	f.reads = append(f.reads, filter)
	if f.readErr != nil {
		return nil, f.readErr
	}
	// Mirrors the server: a Read with no object at all is a validation error,
	// which is how the Children reverse lookup was originally wrong.
	if filter.Object == "" {
		return nil, errors.New("validation_error: object type field is required")
	}
	var out []fga.TupleRecord
	for _, t := range f.tuples {
		if filter.User != "" && filter.User != t.User {
			continue
		}
		if filter.Relation != "" && filter.Relation != t.Relation {
			continue
		}
		// An object of "node:" matches every node, as the real API does.
		if filter.Object != "" && filter.Object != t.Object && filter.Object != TypeNode+":" {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func newTestAuthorizer(t *testing.T, f *fakeClient) *Authorizer {
	t.Helper()
	a, err := New(f)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestNewRejectsNilClient(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected an error for a nil client")
	}
}

func TestCanRejectsAnAssignableRole(t *testing.T) {
	// Checking "editor" instead of "can_edit" returns false for the owner of
	// the file, which reads as a permissions bug rather than a caller mistake.
	a := newTestAuthorizer(t, &fakeClient{})
	for _, role := range []string{RoleOwner, RoleEditor, RoleViewer, RelationParent, "nonsense"} {
		if _, err := a.Can(context.Background(), "u1", role, "n1"); err == nil {
			t.Errorf("%q: expected an error, got nil", role)
		}
	}
}

func TestCanRequiresBothIDs(t *testing.T) {
	a := newTestAuthorizer(t, &fakeClient{})
	if _, err := a.Can(context.Background(), "", PermCanView, "n1"); err == nil {
		t.Error("expected an error for an empty userID")
	}
	if _, err := a.Can(context.Background(), "u1", PermCanView, ""); err == nil {
		t.Error("expected an error for an empty nodeID")
	}
}

func TestCanHelpersSendTypedSubjects(t *testing.T) {
	cases := []struct {
		name     string
		call     func(*Authorizer) (bool, error)
		relation string
	}{
		{"view", func(a *Authorizer) (bool, error) { return a.CanView(context.Background(), "u1", "n1") }, PermCanView},
		{"edit", func(a *Authorizer) (bool, error) { return a.CanEdit(context.Background(), "u1", "n1") }, PermCanEdit},
		{"manage", func(a *Authorizer) (bool, error) { return a.CanManage(context.Background(), "u1", "n1") }, PermCanManage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeClient{allow: map[string]bool{tc.relation + ":node:n1": true}}
			ok, err := tc.call(newTestAuthorizer(t, f))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Error("expected allowed")
			}
			if f.checkUser != "user:u1" {
				t.Errorf("subject = %q, want user:u1", f.checkUser)
			}
			if got := f.checks[0].Object; got != "node:n1" {
				t.Errorf("object = %q, want node:n1", got)
			}
		})
	}
}

func TestCanViaLinkUsesTheLinkSubject(t *testing.T) {
	f := &fakeClient{allow: map[string]bool{PermCanEdit + ":node:n1": true}}
	ok, err := newTestAuthorizer(t, f).CanViaLink(context.Background(), "lnk1", PermCanEdit, "n1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if f.checkUser != "link:lnk1" {
		t.Errorf("subject = %q, want link:lnk1", f.checkUser)
	}
}

func TestPermissionsIsOneRoundTrip(t *testing.T) {
	f := &fakeClient{allow: map[string]bool{
		PermCanView + ":node:n1": true,
		PermCanEdit + ":node:n1": true,
	}}
	got, err := newTestAuthorizer(t, f).Permissions(context.Background(), "u1", "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Permissions{CanView: true, CanEdit: true, CanManage: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if len(f.checks) != 3 {
		t.Errorf("sent %d checks, want 3 in a single batch", len(f.checks))
	}
}

func TestPermissionsForNodesCollapsesDuplicatesAndIncludesDenials(t *testing.T) {
	f := &fakeClient{allow: map[string]bool{PermCanView + ":node:a": true}}
	got, err := newTestAuthorizer(t, f).PermissionsForNodes(context.Background(), "u1", []string{"a", "b", "a", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2 (duplicate and empty id dropped)", len(got))
	}
	if !got["a"].CanView {
		t.Error("a should be viewable")
	}
	if got["b"] != (Permissions{}) {
		t.Errorf("b should be present and entirely false, got %+v", got["b"])
	}
	if len(f.checks) != 6 {
		t.Errorf("sent %d checks, want 6 (2 nodes x 3 permissions)", len(f.checks))
	}
}

func TestListStripsTheNodePrefix(t *testing.T) {
	// Callers hand these straight to a Mongo query, where "node:abc" matches
	// nothing and fails silently as an empty folder.
	f := &fakeClient{objects: []string{"node:abc", "node:def"}}
	got, err := newTestAuthorizer(t, f).ListViewable(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"abc", "def"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if f.listCalled.relation != PermCanView || f.listCalled.objectType != TypeNode {
		t.Errorf("listed %+v, want can_view on node", f.listCalled)
	}
}

func TestShareRejectsAComputedPermission(t *testing.T) {
	// can_edit is derived, so a tuple granting it is accepted by nothing and
	// would fail deep inside OpenFGA with a validation error.
	a := newTestAuthorizer(t, &fakeClient{})
	for _, r := range []string{PermCanView, PermCanEdit, PermCanManage, "nonsense"} {
		if err := a.Share(context.Background(), "n1", "u1", r); err == nil {
			t.Errorf("%q: expected an error, got nil", r)
		}
	}
}

func TestShareWritesTheRoleTuple(t *testing.T) {
	f := &fakeClient{}
	if err := newTestAuthorizer(t, f).Share(context.Background(), "n1", "u1", RoleEditor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := fga.TupleRequest{User: "user:u1", Relation: RoleEditor, Object: "node:n1"}
	if len(f.writes) != 1 || f.writes[0] != want {
		t.Errorf("wrote %+v, want %+v", f.writes, want)
	}
}

func TestReplaceClearsTheOtherRolesTheUserActuallyHolds(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: RoleOwner, Object: "node:n1"},
		{User: "user:u1", Relation: RoleEditor, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).Replace(context.Background(), "n1", "u1", RoleViewer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 1 || f.writes[0].Relation != RoleViewer {
		t.Fatalf("writes = %+v, want a single viewer grant", f.writes)
	}
	var cleared []string
	for _, d := range f.deletes {
		cleared = append(cleared, d.Relation)
	}
	sort.Strings(cleared)
	if want := []string{RoleEditor, RoleOwner}; !reflect.DeepEqual(cleared, want) {
		t.Errorf("cleared %v, want %v", cleared, want)
	}
}

// Deleting a tuple that is not there costs a second of SDK backoff, so Replace
// and RevokeUser must not issue deletes speculatively.
func TestReplaceTouchesNothingItDoesNotNeedTo(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: RoleViewer, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).Replace(context.Background(), "n1", "u1", RoleViewer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 0 {
		t.Errorf("wrote %+v, want nothing: the role is already held", f.writes)
	}
	if len(f.deletes) != 0 {
		t.Errorf("deleted %+v, want nothing: no other role is held", f.deletes)
	}
}

func TestRevokeUserDeletesOnlyTheRolesHeld(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: RoleEditor, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).RevokeUser(context.Background(), "n1", "u1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.deletes) != 1 || f.deletes[0].Relation != RoleEditor {
		t.Errorf("deletes = %+v, want only the editor tuple", f.deletes)
	}
}

func TestRevokeUserOnAStrangerTouchesNothing(t *testing.T) {
	f := &fakeClient{}
	if err := newTestAuthorizer(t, f).RevokeUser(context.Background(), "n1", "stranger"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.deletes) != 0 {
		t.Errorf("deleted %+v, want nothing", f.deletes)
	}
}

func TestShareWithEveryoneUsesTheWildcard(t *testing.T) {
	f := &fakeClient{}
	if err := newTestAuthorizer(t, f).ShareWithEveryone(context.Background(), "n1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.writes[0].User != "user:*" || f.writes[0].Relation != RoleViewer {
		t.Errorf("wrote %+v, want user:* as viewer", f.writes[0])
	}
}

func TestShareLinkRefusesOwner(t *testing.T) {
	// An owner link could reshare and delete, so the door stays shut.
	a := newTestAuthorizer(t, &fakeClient{})
	if err := a.ShareLink(context.Background(), "n1", "lnk1", RoleOwner); err == nil {
		t.Fatal("expected an error for an owner link")
	}
	if err := a.ShareLink(context.Background(), "n1", "lnk1", RoleEditor); err != nil {
		t.Errorf("editor link should be allowed: %v", err)
	}
}

func TestCreateNodeWritesOwnerAndParentTogether(t *testing.T) {
	f := &fakeClient{}
	if err := newTestAuthorizer(t, f).CreateNode(context.Background(), "child", "u1", "folder"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 2 {
		t.Fatalf("wrote %d tuples, want 2 in one call", len(f.writes))
	}
	if f.writes[0] != (fga.TupleRequest{User: "user:u1", Relation: RoleOwner, Object: "node:child"}) {
		t.Errorf("owner tuple = %+v", f.writes[0])
	}
	if f.writes[1] != (fga.TupleRequest{User: "node:folder", Relation: RelationParent, Object: "node:child"}) {
		t.Errorf("parent tuple = %+v", f.writes[1])
	}
}

func TestCreateNodeAtRootOmitsTheParentTuple(t *testing.T) {
	f := &fakeClient{}
	if err := newTestAuthorizer(t, f).CreateNode(context.Background(), "top", "u1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 1 {
		t.Errorf("wrote %d tuples, want 1", len(f.writes))
	}
}

func TestSetParentRefusesSelfParent(t *testing.T) {
	// OpenFGA accepts the tuple and then loops resolving can_view through it.
	a := newTestAuthorizer(t, &fakeClient{})
	if err := a.SetParent(context.Background(), "n1", "n1"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSetParentWritesBeforeRemovingTheOldLink(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "node:old", Relation: RelationParent, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).SetParent(context.Background(), "n1", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 1 || f.writes[0].User != "node:new" {
		t.Fatalf("writes = %+v, want the new parent", f.writes)
	}
	if len(f.deletes) != 1 || f.deletes[0].User != "node:old" {
		t.Errorf("deletes = %+v, want only the old parent", f.deletes)
	}
}

func TestSetParentToRootRemovesEveryParentLink(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "node:old", Relation: RelationParent, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).SetParent(context.Background(), "n1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.writes) != 0 {
		t.Errorf("wrote %+v, want nothing", f.writes)
	}
	if len(f.deletes) != 1 {
		t.Errorf("deletes = %+v, want the old parent removed", f.deletes)
	}
}

func TestSetParentIsANoOpWhenTheParentIsUnchanged(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "node:same", Relation: RelationParent, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).SetParent(context.Background(), "n1", "same"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.deletes) != 0 {
		t.Errorf("deleted %+v, want the existing link left in place", f.deletes)
	}
}

func TestGrantsClassifiesSubjectsAndSkipsTheParentLink(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: RoleOwner, Object: "node:n1"},
		{User: "user:u2", Relation: RoleViewer, Object: "node:n1"},
		{User: "link:lnk1", Relation: RoleEditor, Object: "node:n1"},
		{User: "user:*", Relation: RoleViewer, Object: "node:n1"},
		{User: "node:folder", Relation: RelationParent, Object: "node:n1"},
	}}
	got, err := newTestAuthorizer(t, f).Grants(context.Background(), "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d grants, want 4 (the parent link is not a grant)", len(got))
	}

	byRole := map[string]Grant{}
	for _, g := range got {
		byRole[g.Subject] = g
	}
	if byRole["user:u1"].UserID != "u1" || byRole["user:u1"].Role != RoleOwner {
		t.Errorf("u1 = %+v", byRole["user:u1"])
	}
	if byRole["link:lnk1"].LinkID != "lnk1" {
		t.Errorf("link = %+v", byRole["link:lnk1"])
	}
	if !byRole["user:*"].Public {
		t.Errorf("wildcard should be marked public, got %+v", byRole["user:*"])
	}
	if byRole["user:*"].UserID != "" {
		t.Errorf("wildcard should not carry a user id, got %q", byRole["user:*"].UserID)
	}
}

func TestGrantsSurvivesARelationAddedToTheModelLater(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: "commenter", Object: "node:n1"},
	}}
	got, err := newTestAuthorizer(t, f).Grants(context.Background(), "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want an unknown relation ignored rather than mislabelled", got)
	}
}

func TestDeleteNodeRemovesEveryTuplePointingAtIt(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "user:u1", Relation: RoleOwner, Object: "node:n1"},
		{User: "node:folder", Relation: RelationParent, Object: "node:n1"},
	}}
	if err := newTestAuthorizer(t, f).DeleteNode(context.Background(), "n1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.deletes) != 2 {
		t.Errorf("deleted %d tuples, want 2 including the parent link", len(f.deletes))
	}
}

func TestChildrenReadsTheReverseDirection(t *testing.T) {
	f := &fakeClient{tuples: []fga.TupleRecord{
		{User: "node:folder", Relation: RelationParent, Object: "node:a"},
		{User: "node:folder", Relation: RelationParent, Object: "node:b"},
	}}
	a := newTestAuthorizer(t, f)
	got, err := a.Children(context.Background(), "folder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if f.reads[0].Object != TypeNode+":" {
		t.Errorf("read object = %q, want the bare type; OpenFGA rejects an empty object", f.reads[0].Object)
	}
}

// A transport error that is not OpenFGA's invalid-input code must reach the
// caller: swallowing it would report a share as applied when it was not.
func TestWriteSurfacesAnUnrelatedError(t *testing.T) {
	boom := errors.New("connection refused")
	f := &fakeClient{writeErr: boom}
	err := newTestAuthorizer(t, f).Share(context.Background(), "n1", "u1", RoleViewer)
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want the transport error", err)
	}
	if len(f.reads) != 0 {
		t.Error("should not read back for an error that is not invalid-input")
	}
}

func TestDeleteSurfacesAnUnrelatedError(t *testing.T) {
	boom := errors.New("connection refused")
	f := &fakeClient{deleteErr: boom}
	err := newTestAuthorizer(t, f).Unshare(context.Background(), "n1", "u1", RoleViewer)
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want the transport error", err)
	}
}

func TestRequiredArgumentsAreChecked(t *testing.T) {
	a := newTestAuthorizer(t, &fakeClient{})
	ctx := context.Background()
	cases := map[string]error{
		"Share no node":       a.Share(ctx, "", "u1", RoleViewer),
		"Share no user":       a.Share(ctx, "n1", "", RoleViewer),
		"Unshare no node":     a.Unshare(ctx, "", "u1", RoleViewer),
		"RevokeUser no user":  a.RevokeUser(ctx, "n1", ""),
		"ShareEveryone":       a.ShareWithEveryone(ctx, ""),
		"UnshareEveryone":     a.UnshareWithEveryone(ctx, ""),
		"ShareLink no link":   a.ShareLink(ctx, "n1", "", RoleViewer),
		"RevokeLink no node":  a.RevokeLink(ctx, "", "lnk", RoleViewer),
		"CreateNode no owner": a.CreateNode(ctx, "n1", "", ""),
		"CreateNode no node":  a.CreateNode(ctx, "", "u1", ""),
		"SetParent no node":   a.SetParent(ctx, "", "p"),
		"DeleteNode no node":  a.DeleteNode(ctx, ""),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		} else if !strings.HasPrefix(err.Error(), "authz:") {
			t.Errorf("%s: error should be prefixed authz:, got %v", name, err)
		}
	}
}

func TestModelShipsWithThePackage(t *testing.T) {
	if !strings.Contains(ModelDSL(), "define can_view") {
		t.Error("model.fga is missing can_view")
	}
	if len(ModelJSON()) == 0 {
		t.Fatal("model.json is empty")
	}
	// ModelJSON hands out a copy; a caller mutating it must not corrupt the
	// embedded bytes for the next caller.
	first := ModelJSON()
	first[0] = 'X'
	if ModelJSON()[0] == 'X' {
		t.Error("ModelJSON returned a reference to the embedded bytes")
	}
}

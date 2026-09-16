package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/LooneY2K/common-pkg-svc/fga"
)

// These run against a real OpenFGA and are the only tests that prove the
// model actually behaves as the package claims — inheritance through parent
// folders, the owner implying edit, the wildcard, and what a link can reach.
// A fake cannot prove any of it without reimplementing OpenFGA's evaluator.
//
// Point AUTHZ_IT_URL at a server and they run; leave it unset and they skip:
//
//	AUTHZ_IT_URL=http://localhost:8088 go test ./authz/ -run Integration -v
//
// Each run creates its own store and uploads the embedded model, so it never
// touches an existing one and needs no fixture.
func integrationAuthorizer(t *testing.T) *Authorizer {
	t.Helper()

	url := os.Getenv("AUTHZ_IT_URL")
	if url == "" {
		t.Skip("set AUTHZ_IT_URL to run the integration tests")
	}

	storeID := createStore(t, url)
	// Each test gets its own store so tuples cannot bleed between them, and
	// drops it afterwards so a full run does not leave fifty behind.
	t.Cleanup(func() { deleteStore(t, url, storeID) })

	modelID := uploadModel(t, url, storeID)

	client, err := fga.NewClientNoAuth(url, storeID, modelID)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	a, err := New(client)
	if err != nil {
		t.Fatalf("authorizer: %v", err)
	}
	return a
}

func createStore(t *testing.T, baseURL string) string {
	t.Helper()
	name := fmt.Sprintf("authz-it-%d", time.Now().UnixNano())
	var out struct {
		ID string `json:"id"`
	}
	post(t, baseURL+"/stores", map[string]string{"name": name}, &out)
	if out.ID == "" {
		t.Fatal("store creation returned no id")
	}
	return out.ID
}

func uploadModel(t *testing.T, baseURL, storeID string) string {
	t.Helper()
	var model map[string]any
	if err := json.Unmarshal(ModelJSON(), &model); err != nil {
		t.Fatalf("embedded model.json is not valid JSON: %v", err)
	}
	var out struct {
		ID string `json:"authorization_model_id"`
	}
	post(t, baseURL+"/stores/"+storeID+"/authorization-models", model, &out)
	if out.ID == "" {
		t.Fatal("model upload returned no id")
	}
	return out.ID
}

func deleteStore(t *testing.T, baseURL, storeID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/stores/"+storeID, nil)
	if err != nil {
		t.Logf("could not build delete request for store %s: %v", storeID, err)
		return
	}
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Logf("could not delete store %s: %v", storeID, err)
		return
	}
	res.Body.Close()
}

func post(t *testing.T, url string, body, into any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("content-type", "application/json")

	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		var buf bytes.Buffer
		buf.ReadFrom(res.Body)
		t.Fatalf("POST %s: %d %s", url, res.StatusCode, buf.String())
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func TestIntegrationOwnerHasEveryPermission(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	perms, err := a.Permissions(ctx, "alice", "file1")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if want := (Permissions{true, true, true}); perms != want {
		t.Errorf("owner got %+v, want %+v", perms, want)
	}

	// The point of checking permissions rather than roles: alice holds only an
	// `owner` tuple, yet can_edit must still be true.
	if ok, err := a.CanEdit(ctx, "alice", "file1"); err != nil || !ok {
		t.Errorf("owner should be able to edit: ok=%v err=%v", ok, err)
	}
}

func TestIntegrationStrangerHasNothing(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	perms, err := a.Permissions(ctx, "bob", "file1")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if perms != (Permissions{}) {
		t.Errorf("stranger got %+v, want nothing", perms)
	}
}

func TestIntegrationAccessInheritsThroughNestedFolders(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	// alice: /top/mid/deep.txt
	for _, n := range []struct{ id, parent string }{
		{"top", ""},
		{"mid", "top"},
		{"deep", "mid"},
	} {
		if err := a.CreateNode(ctx, n.id, "alice", n.parent); err != nil {
			t.Fatalf("CreateNode %s: %v", n.id, err)
		}
	}

	// One grant at the top must reach two levels down.
	if err := a.Share(ctx, "top", "bob", RoleViewer); err != nil {
		t.Fatalf("Share: %v", err)
	}

	perms, err := a.Permissions(ctx, "bob", "deep")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if want := (Permissions{CanView: true}); perms != want {
		t.Errorf("inherited viewer got %+v, want %+v", perms, want)
	}

	// Promoting the folder grant must promote the leaf too.
	if err := a.Replace(ctx, "top", "bob", RoleEditor); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	perms, err = a.Permissions(ctx, "bob", "deep")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if !perms.CanEdit {
		t.Errorf("inherited editor got %+v, want can_edit", perms)
	}
	if perms.CanManage {
		t.Error("an editor must not be able to manage")
	}
}

func TestIntegrationReplaceDemotesInsteadOfAccumulating(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := a.Share(ctx, "file1", "bob", RoleEditor); err != nil {
		t.Fatalf("Share: %v", err)
	}

	// Share alone would leave the editor tuple in place and bob would keep
	// write access after being "demoted" in the UI.
	if err := a.Replace(ctx, "file1", "bob", RoleViewer); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	perms, err := a.Permissions(ctx, "bob", "file1")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if want := (Permissions{CanView: true}); perms != want {
		t.Errorf("after demotion got %+v, want view only", perms)
	}
}

func TestIntegrationRevokeRemovesDirectAccessOnly(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	if err := a.CreateNode(ctx, "file1", "alice", "folder"); err != nil {
		t.Fatalf("CreateNode file: %v", err)
	}
	if err := a.Share(ctx, "folder", "bob", RoleViewer); err != nil {
		t.Fatalf("Share folder: %v", err)
	}
	if err := a.Share(ctx, "file1", "bob", RoleEditor); err != nil {
		t.Fatalf("Share file: %v", err)
	}

	if err := a.RevokeUser(ctx, "file1", "bob"); err != nil {
		t.Fatalf("RevokeUser: %v", err)
	}

	// Edit access is gone, but the folder grant still grants read — revoking
	// on a file cannot undo a grant made on its parent.
	perms, err := a.Permissions(ctx, "bob", "file1")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if want := (Permissions{CanView: true}); perms != want {
		t.Errorf("got %+v, want inherited view surviving a direct revoke", perms)
	}
}

func TestIntegrationMovingANodeMovesItsInheritedAccess(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	for _, id := range []string{"shared", "private"} {
		if err := a.CreateNode(ctx, id, "alice", ""); err != nil {
			t.Fatalf("CreateNode %s: %v", id, err)
		}
	}
	if err := a.CreateNode(ctx, "file1", "alice", "shared"); err != nil {
		t.Fatalf("CreateNode file: %v", err)
	}
	if err := a.Share(ctx, "shared", "bob", RoleViewer); err != nil {
		t.Fatalf("Share: %v", err)
	}

	if ok, _ := a.CanView(ctx, "bob", "file1"); !ok {
		t.Fatal("bob should see the file while it is in the shared folder")
	}

	if err := a.SetParent(ctx, "file1", "private"); err != nil {
		t.Fatalf("SetParent: %v", err)
	}

	if ok, err := a.CanView(ctx, "bob", "file1"); err != nil {
		t.Fatalf("CanView: %v", err)
	} else if ok {
		// If the old parent tuple survived, the file would leak out of the
		// folder it was moved into.
		t.Error("bob should lose access once the file leaves the shared folder")
	}

	parent, err := a.Parent(ctx, "file1")
	if err != nil {
		t.Fatalf("Parent: %v", err)
	}
	if parent != "private" {
		t.Errorf("parent = %q, want private", parent)
	}
}

func TestIntegrationEveryoneGrantsReadToAnyUser(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := a.ShareWithEveryone(ctx, "file1"); err != nil {
		t.Fatalf("ShareWithEveryone: %v", err)
	}

	perms, err := a.Permissions(ctx, "nobody-in-particular", "file1")
	if err != nil {
		t.Fatalf("Permissions: %v", err)
	}
	if !perms.CanView {
		t.Error("a public node should be readable by any user")
	}
	if perms.CanEdit {
		t.Error("public access must never imply write")
	}

	if err := a.UnshareWithEveryone(ctx, "file1"); err != nil {
		t.Fatalf("UnshareWithEveryone: %v", err)
	}
	if ok, _ := a.CanView(ctx, "nobody-in-particular", "file1"); ok {
		t.Error("access should be gone once the wildcard is withdrawn")
	}
}

func TestIntegrationLinkAccessAndRevocation(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	if err := a.CreateNode(ctx, "file1", "alice", "folder"); err != nil {
		t.Fatalf("CreateNode file: %v", err)
	}
	if err := a.ShareLink(ctx, "folder", "lnk1", RoleViewer); err != nil {
		t.Fatalf("ShareLink: %v", err)
	}

	// A link on a folder reaches the files inside it, like any other subject.
	if ok, err := a.CanViaLink(ctx, "lnk1", PermCanView, "file1"); err != nil || !ok {
		t.Errorf("link should reach the file: ok=%v err=%v", ok, err)
	}
	if ok, _ := a.CanViaLink(ctx, "lnk1", PermCanEdit, "file1"); ok {
		t.Error("a viewer link must not grant edit")
	}
	if ok, _ := a.CanViaLink(ctx, "lnk2", PermCanView, "file1"); ok {
		t.Error("an unknown link must grant nothing")
	}

	if err := a.RevokeLink(ctx, "folder", "lnk1", RoleViewer); err != nil {
		t.Fatalf("RevokeLink: %v", err)
	}
	if ok, _ := a.CanViaLink(ctx, "lnk1", PermCanView, "file1"); ok {
		t.Error("a revoked link must grant nothing")
	}
}

func TestIntegrationGrantsAndDeleteAreIdempotent(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// A double-click on Share, and a re-run of the same create, must both be
	// successes rather than 400s surfaced to the user.
	if err := a.Share(ctx, "file1", "bob", RoleViewer); err != nil {
		t.Fatalf("first share: %v", err)
	}
	if err := a.Share(ctx, "file1", "bob", RoleViewer); err != nil {
		t.Errorf("re-granting an existing role should succeed: %v", err)
	}
	if err := a.CreateNode(ctx, "file1", "alice", ""); err != nil {
		t.Errorf("re-creating an existing node should succeed: %v", err)
	}

	if err := a.Unshare(ctx, "file1", "bob", RoleViewer); err != nil {
		t.Fatalf("first unshare: %v", err)
	}
	if err := a.Unshare(ctx, "file1", "bob", RoleViewer); err != nil {
		t.Errorf("revoking a role that is not held should succeed: %v", err)
	}
	if err := a.Unshare(ctx, "file1", "never-shared", RoleEditor); err != nil {
		t.Errorf("revoking from a stranger should succeed: %v", err)
	}
}

func TestIntegrationGrantsListsTheShareDialog(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	if err := a.CreateNode(ctx, "file1", "alice", "folder"); err != nil {
		t.Fatalf("CreateNode file: %v", err)
	}
	if err := a.Share(ctx, "file1", "bob", RoleEditor); err != nil {
		t.Fatalf("Share: %v", err)
	}
	if err := a.ShareWithEveryone(ctx, "file1"); err != nil {
		t.Fatalf("ShareWithEveryone: %v", err)
	}
	if err := a.ShareLink(ctx, "file1", "lnk1", RoleViewer); err != nil {
		t.Fatalf("ShareLink: %v", err)
	}

	grants, err := a.Grants(ctx, "file1")
	if err != nil {
		t.Fatalf("Grants: %v", err)
	}

	var subjects []string
	for _, g := range grants {
		subjects = append(subjects, g.Subject+"="+g.Role)
	}
	sort.Strings(subjects)

	want := []string{"link:lnk1=viewer", "user:*=viewer", "user:alice=owner", "user:bob=editor"}
	if len(subjects) != len(want) {
		t.Fatalf("got %v, want %v (the parent link is not a grant)", subjects, want)
	}
	for i := range want {
		if subjects[i] != want[i] {
			t.Errorf("got %v, want %v", subjects, want)
			break
		}
	}
}

func TestIntegrationListViewableReturnsBareIDs(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	if err := a.CreateNode(ctx, "inside", "alice", "folder"); err != nil {
		t.Fatalf("CreateNode inside: %v", err)
	}
	if err := a.CreateNode(ctx, "elsewhere", "alice", ""); err != nil {
		t.Fatalf("CreateNode elsewhere: %v", err)
	}
	if err := a.Share(ctx, "folder", "bob", RoleViewer); err != nil {
		t.Fatalf("Share: %v", err)
	}

	ids, err := a.ListViewable(ctx, "bob")
	if err != nil {
		t.Fatalf("ListViewable: %v", err)
	}
	sort.Strings(ids)

	// The folder and what it contains, and nothing else.
	if want := []string{"folder", "inside"}; len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Errorf("got %v, want %v", ids, want)
	}
}

func TestIntegrationDeleteNodeClearsEveryTuple(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	if err := a.CreateNode(ctx, "file1", "alice", "folder"); err != nil {
		t.Fatalf("CreateNode file: %v", err)
	}
	if err := a.Share(ctx, "file1", "bob", RoleEditor); err != nil {
		t.Fatalf("Share: %v", err)
	}

	if err := a.DeleteNode(ctx, "file1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	grants, err := a.Grants(ctx, "file1")
	if err != nil {
		t.Fatalf("Grants: %v", err)
	}
	if len(grants) != 0 {
		t.Errorf("got %+v, want no grants left", grants)
	}
	if parent, _ := a.Parent(ctx, "file1"); parent != "" {
		t.Errorf("parent = %q, want the link removed too", parent)
	}
	// A stale grant would resurrect with the id if it were ever reused.
	if ok, _ := a.CanEdit(ctx, "bob", "file1"); ok {
		t.Error("bob should have nothing on a deleted node")
	}
	if ok, _ := a.CanManage(ctx, "alice", "file1"); ok {
		t.Error("alice should have nothing on a deleted node")
	}
}

func TestIntegrationChildrenFindsTheSubtree(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "folder", "alice", ""); err != nil {
		t.Fatalf("CreateNode folder: %v", err)
	}
	for _, id := range []string{"a", "b"} {
		if err := a.CreateNode(ctx, id, "alice", "folder"); err != nil {
			t.Fatalf("CreateNode %s: %v", id, err)
		}
	}

	kids, err := a.Children(ctx, "folder")
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	sort.Strings(kids)
	if want := []string{"a", "b"}; len(kids) != 2 || kids[0] != want[0] || kids[1] != want[1] {
		t.Errorf("got %v, want %v", kids, want)
	}
}

func TestIntegrationPermissionsForNodesMatchesOneByOne(t *testing.T) {
	a := integrationAuthorizer(t)
	ctx := context.Background()

	if err := a.CreateNode(ctx, "owned", "bob", ""); err != nil {
		t.Fatalf("CreateNode owned: %v", err)
	}
	if err := a.CreateNode(ctx, "shared", "alice", ""); err != nil {
		t.Fatalf("CreateNode shared: %v", err)
	}
	if err := a.CreateNode(ctx, "hidden", "alice", ""); err != nil {
		t.Fatalf("CreateNode hidden: %v", err)
	}
	if err := a.Share(ctx, "shared", "bob", RoleViewer); err != nil {
		t.Fatalf("Share: %v", err)
	}

	ids := []string{"owned", "shared", "hidden"}
	batch, err := a.PermissionsForNodes(ctx, "bob", ids)
	if err != nil {
		t.Fatalf("PermissionsForNodes: %v", err)
	}

	for _, id := range ids {
		single, err := a.Permissions(ctx, "bob", id)
		if err != nil {
			t.Fatalf("Permissions %s: %v", id, err)
		}
		if batch[id] != single {
			t.Errorf("%s: batch %+v != single %+v", id, batch[id], single)
		}
	}
}

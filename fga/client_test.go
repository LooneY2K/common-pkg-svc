// client_test.go covers the construction-time validation of SDKClient.
// The full request-path behaviour (Check/BatchCheck/Read against a live
// OpenFGA server) is covered by integration tests in each consuming service;
// here we focus on the bits that don't require a live FGA server.
package fga

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testStoreID = "01HZ8Y0J3RJN0Y3FYTC8RZ7K7T"
	testModelID = "01HZ8Y0J3RJN0Y3FYTC8RZ7K7V"
)

// fakeServer answers every request with reply and records each body by path.
func fakeServer(t *testing.T, reply string) (*SDKClient, map[string]map[string]any) {
	t.Helper()
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies[r.URL.Path] = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClientNoAuth(srv.URL, testStoreID, testModelID)
	if err != nil {
		t.Fatal(err)
	}
	return c, bodies
}

func TestNewClient_RejectsMissingParams(t *testing.T) {
	cases := []struct {
		name string
		args [4]string // url, store, model, token
	}{
		{"missing url", [4]string{"", "01J0", "01J0", "tok"}},
		{"missing store", [4]string{"http://x", "", "01J0", "tok"}},
		{"missing model", [4]string{"http://x", "01J0", "", "tok"}},
		{"missing token", [4]string{"http://x", "01J0", "01J0", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.args[0], tc.args[1], tc.args[2], tc.args[3])
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "fga:") {
				t.Fatalf("expected error prefix 'fga:', got %v", err)
			}
		})
	}
}

func TestNewClient_RejectsBadULID(t *testing.T) {
	// StoreId/AuthorizationModelId must be ULID-shaped per the SDK; provide
	// obviously non-ULID values and ensure the constructor rejects them
	// (rather than silently producing a client that 4xxs at first call).
	_, err := NewClient("http://x", "not-a-ulid", "01HZ8Y0J3RJN0Y3FYTC8RZ7K7T", "tok")
	if err == nil {
		t.Fatalf("expected ULID validation error")
	}
}

func TestSubjectHelpers(t *testing.T) {
	// Tight contract check: subject helpers must format as "<type>:<id>".
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"user", UserSubject("u1"), "user:u1"},
		{"org", OrganisationSubject("o1"), "organisation:o1"},
		{"project", ProjectSubject("p1"), "project:p1"},
		{"survey", SurveySubject("s1"), "survey:s1"},
		{"workspace", WorkspaceSubject("w1"), "workspace:w1"},
		{"team", TeamSubject("t1"), "team:t1"},
		{"userset", Userset("team", "t1", "maintainer"), "team:t1#maintainer"},
		{"team members", TeamMembers("t1"), "team:t1#member"},
		{"organisation members", OrganisationMembers("o1"), "organisation:o1#member"},
		{"generic", Subject("widget", "42"), "widget:42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, tc.got)
			}
		})
	}
}

func TestRoleValidators(t *testing.T) {
	if !IsValidProjectRole(RoleProjectManager) {
		t.Fatal("project_manager should be valid project role")
	}
	if IsValidProjectRole(RoleSuperAdmin) {
		t.Fatal("super_admin is org-scope, must NOT be valid project role")
	}
	if !IsValidOrgRole(RoleSuperAdmin) {
		t.Fatal("super_admin should be valid org role")
	}
	if IsValidOrgRole(RoleProjectManager) {
		t.Fatal("project_manager is project-scope, must NOT be valid org role")
	}
	if !IsClientRole(RoleClientAdmin) {
		t.Fatal("client_admin should be a client role")
	}
	if IsClientRole(RoleProjectManager) {
		t.Fatal("project_manager is NOT a client role")
	}
	if !IsValidWorkspaceRole(RoleWorkspaceAdmin) || !IsValidWorkspaceRole(RoleClientViewer) {
		t.Fatal("admin and client_viewer should be valid workspace roles")
	}
	if IsValidWorkspaceRole(RoleProjectManager) {
		t.Fatal("project_manager is held through workspace admin, not granted on a workspace")
	}
	if !IsValidTeamRole(RoleTeamMaintainer) || IsValidTeamRole(RoleOrgAdmin) {
		t.Fatal("maintainer is a team role and org_admin is not")
	}
}

func TestApplyAsksTheServerToIgnoreDuplicatesAndMissingTuples(t *testing.T) {
	c, bodies := fakeServer(t, `{}`)
	err := c.Apply(context.Background(),
		[]TupleRequest{{User: "team:t1#member", Relation: "reviewer", Object: "workspace:w1"}},
		[]TupleRequest{{User: "user:u1", Relation: "reviewer", Object: "workspace:w1"}})
	if err != nil {
		t.Fatal(err)
	}
	body := bodies["/stores/"+testStoreID+"/write"]
	if body["authorization_model_id"] != testModelID {
		t.Fatalf("model not pinned: %v", body)
	}
	writes := body["writes"].(map[string]any)
	deletes := body["deletes"].(map[string]any)
	if writes["on_duplicate"] != "ignore" || deletes["on_missing"] != "ignore" {
		t.Fatalf("writes %v, deletes %v", writes, deletes)
	}
	key := writes["tuple_keys"].([]any)[0].(map[string]any)
	if key["user"] != "team:t1#member" || key["object"] != "workspace:w1" {
		t.Fatalf("write key %v", key)
	}
}

func TestApplyRefusesMoreThanOneWriteCanCarry(t *testing.T) {
	c, bodies := fakeServer(t, `{}`)
	tuples := make([]TupleRequest, 101)
	for i := range tuples {
		tuples[i] = TupleRequest{User: fmt.Sprintf("user:u%d", i), Relation: "member", Object: "team:t1"}
	}
	if err := c.Apply(context.Background(), tuples, nil); err == nil {
		t.Fatal("expected an error for 101 tuples")
	}
	if len(bodies) != 0 {
		t.Fatalf("nothing should reach the server, got %v", bodies)
	}
}

func TestDeleteTuplesChunksAtOneHundred(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	c, err := NewClientNoAuth(srv.URL, testStoreID, testModelID)
	if err != nil {
		t.Fatal(err)
	}
	tuples := make([]TupleRequest, 150)
	for i := range tuples {
		tuples[i] = TupleRequest{User: fmt.Sprintf("user:u%d", i), Relation: "member", Object: "team:t1"}
	}
	if err := c.DeleteTuples(context.Background(), tuples); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("want 2 writes, got %d", calls)
	}
}

func TestListUsersReturnsTypedUsersAndSkipsUsersets(t *testing.T) {
	c, bodies := fakeServer(t, `{"users":[{"object":{"type":"user","id":"u1"}},{"userset":{"type":"team","id":"t1","relation":"member"}}]}`)
	users, err := c.ListUsers(context.Background(), "project:p1", "survey_manager", "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0] != "user:u1" {
		t.Fatalf("users %v", users)
	}
	body := bodies["/stores/"+testStoreID+"/list-users"]
	object := body["object"].(map[string]any)
	if object["type"] != "project" || object["id"] != "p1" || body["relation"] != "survey_manager" {
		t.Fatalf("request %v", body)
	}
}

func TestListUsersRejectsAnUntypedObject(t *testing.T) {
	c, _ := fakeServer(t, `{}`)
	if _, err := c.ListUsers(context.Background(), "p1", "survey_manager", "user"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAll9ProjectPermissions(t *testing.T) {
	// Stable order matters for FE rendering — assert length and first/last.
	if len(All9ProjectPermissions) != 9 {
		t.Fatalf("expected 9 permissions, got %d", len(All9ProjectPermissions))
	}
	if All9ProjectPermissions[0] != PermCanUpload {
		t.Fatalf("expected first permission %q, got %q", PermCanUpload, All9ProjectPermissions[0])
	}
	if All9ProjectPermissions[len(All9ProjectPermissions)-1] != PermCanCallAPI {
		t.Fatalf("expected last permission %q, got %q", PermCanCallAPI, All9ProjectPermissions[len(All9ProjectPermissions)-1])
	}
}

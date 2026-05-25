// client_test.go covers the construction-time validation of SDKClient.
// The full request-path behaviour (Check/BatchCheck/Read against a live
// OpenFGA server) is covered by integration tests in each consuming service;
// here we focus on the bits that don't require a live FGA server.
package fga

import (
	"strings"
	"testing"
)

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

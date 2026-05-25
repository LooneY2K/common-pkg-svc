package errors

import (
	"net/http"
	"testing"
)

func TestBadRequest(t *testing.T) {
	err := BadRequest("bad input")
	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", err.StatusCode)
	}
	if err.Error() != "bad input" {
		t.Errorf("expected 'bad input', got %q", err.Error())
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("not found")
	if err.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", err.StatusCode)
	}
	if err.Error() != "not found" {
		t.Errorf("expected 'not found', got %q", err.Error())
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("foo")
	if err.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.StatusCode)
	}
	if err.Message != "foo" {
		t.Errorf("expected message 'foo', got %q", err.Message)
	}
	// Verify the standard error interface
	if err.Error() != "foo" {
		t.Errorf("expected Error() 'foo', got %q", err.Error())
	}
}

func TestServiceUnavailable(t *testing.T) {
	err := ServiceUnavailable("bar")
	if err.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", err.StatusCode)
	}
	if err.Message != "bar" {
		t.Errorf("expected message 'bar', got %q", err.Message)
	}
	// Verify the standard error interface
	if err.Error() != "bar" {
		t.Errorf("expected Error() 'bar', got %q", err.Error())
	}
}

func TestConflictImplementsError(t *testing.T) {
	var _ error = Conflict("test")
}

func TestServiceUnavailableImplementsError(t *testing.T) {
	var _ error = ServiceUnavailable("test")
}

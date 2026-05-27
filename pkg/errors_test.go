package pkg

import (
	"net/http"
	"testing"
)

func TestHTTPError_Error(t *testing.T) {
	e := NewHTTPError(http.StatusBadRequest, "bad request")
	if e.Error() != "bad request" {
		t.Errorf("expected 'bad request', got '%s'", e.Error())
	}
	if e.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", e.Code)
	}
}

func TestErrUnauthorized(t *testing.T) {
	e := ErrUnauthorized("no access")
	if e.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", e.Code)
	}
	if e.Error() != "no access" {
		t.Errorf("expected 'no access', got '%s'", e.Error())
	}
}

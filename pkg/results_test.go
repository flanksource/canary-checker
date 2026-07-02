package pkg

import (
	"errors"
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

func TestCheckResultFailureTypes(t *testing.T) {
	canary := v1.Canary{}
	check := v1.HTTPCheck{}

	t.Run("legacy failure is unclassified", func(t *testing.T) {
		result := Success(check, canary)
		result.Failf("expected %d, got %d", 404, 200)

		if result.Pass {
			t.Fatal("expected result to fail")
		}
		if result.FailureType != FailureNone {
			t.Fatalf("expected failure type %q, got %q", FailureNone, result.FailureType)
		}
	})

	t.Run("assertion failure", func(t *testing.T) {
		result := Success(check, canary)
		result.AssertionFailuref("expected %d, got %d", 404, 200)

		if result.Pass {
			t.Fatal("expected result to fail")
		}
		if result.FailureType != FailureAssertion {
			t.Fatalf("expected failure type %q, got %q", FailureAssertion, result.FailureType)
		}
	})

	t.Run("runtime error", func(t *testing.T) {
		result := Success(check, canary)
		err := errors.New("connection refused")
		result.ErrorMessage(err)

		if result.Pass {
			t.Fatal("expected result to fail")
		}
		if result.FailureType != FailureRuntime {
			t.Fatalf("expected failure type %q, got %q", FailureRuntime, result.FailureType)
		}
		if result.ErrorObject != err {
			t.Fatal("expected original error to be preserved")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		result := Success(check, canary)
		result.Invalidf("missing url")

		if !result.Invalid {
			t.Fatal("expected result to be invalid")
		}
		if result.FailureType != FailureInvalid {
			t.Fatalf("expected failure type %q, got %q", FailureInvalid, result.FailureType)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		result := Success(check, canary)
		result.InternalErrorf("failed to persist status")

		if !result.InternalError {
			t.Fatal("expected result to be marked internal error")
		}
		if result.FailureType != FailureInternal {
			t.Fatalf("expected failure type %q, got %q", FailureInternal, result.FailureType)
		}
	})
}

func TestFailureTypeFirstClassificationWins(t *testing.T) {
	result := Success(v1.HTTPCheck{}, v1.Canary{})

	result.AssertionFailuref("expected 404")
	result.ErrorMessage(errors.New("display template failed"))

	if result.FailureType != FailureAssertion {
		t.Fatalf("expected failure type %q, got %q", FailureAssertion, result.FailureType)
	}

	result.RuntimeErrorf("connection refused")
	result.Invalidf("missing url")

	if result.FailureType != FailureAssertion {
		t.Fatalf("expected failure type %q, got %q", FailureAssertion, result.FailureType)
	}
}

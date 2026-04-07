package authcheck

import "testing"

func TestCheckUnknownStrategy(t *testing.T) {
	t.Parallel()

	result := Check("unknown-strategy")
	if result.Available {
		t.Fatalf("expected unavailable for unknown strategy")
	}
}

func TestCheckInheritIsAvailable(t *testing.T) {
	t.Parallel()

	result := Check("inherit")
	if !result.Available {
		t.Fatalf("expected inherit strategy to be available")
	}
}

package ulid

import (
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	id := New()
	if len(id) != 26 {
		t.Fatalf("expected length 26, got %d (%s)", len(id), id)
	}

	if !IsValid(id) {
		t.Fatalf("expected valid ULID, got %s", id)
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"01HZX8M2R6Q5K4P3Y7W9T1B0E2", true},
		{"01ARZ3NDEKTSV4RRFFQ69G5FA", false},  // too short
		{"01ARZ3NDEKTSV4RRFFQ69G5FAVV", false}, // too long
		{"01ARZ3NDEKTSV4RRFFQ69G5FAi", false}, // invalid char 'i'
		{"01ARZ3NDEKTSV4RRFFQ69G5FAu", false}, // invalid char 'u'
		{"", false},
	}

	for _, tt := range tests {
		got := IsValid(tt.input)
		if got != tt.valid {
			t.Errorf("IsValid(%q) = %v, expected %v", tt.input, got, tt.valid)
		}
	}
}

func TestMonotonicityWithinSameMillisecond(t *testing.T) {
	fixedTime := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	id1 := NewAt(fixedTime)
	id2 := NewAt(fixedTime)

	if len(id1) != 26 || len(id2) != 26 {
		t.Fatalf("invalid ULID lengths: %s, %s", id1, id2)
	}

	if id1 >= id2 {
		t.Fatalf("expected id1 < id2 for same timestamp, got id1=%s, id2=%s", id1, id2)
	}
}

func TestTimeOrdering(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	id1 := NewAt(t1)
	id2 := NewAt(t2)

	if strings.Compare(id1, id2) >= 0 {
		t.Fatalf("expected id1 < id2, got id1=%s, id2=%s", id1, id2)
	}
}

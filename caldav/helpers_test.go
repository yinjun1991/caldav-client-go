package caldav

import "testing"

func TestNormalizeCollectionPath(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"root", "/", "/"},
		{"noTrailing", "/cal", "/cal"},
		{"singleTrailing", "/cal/", "/cal"},
		{"multipleTrailing", "/cal////", "/cal"},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCollectionPath(tc.input); got != tc.expected {
				t.Fatalf("normalizeCollectionPath(%q)=%q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestSameCollectionPath(t *testing.T) {
	tcs := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"exact", "/cal", "/cal", true},
		{"trailingA", "/cal/", "/cal", true},
		{"trailingB", "/cal", "/cal///", true},
		{"rootVsSlash", "/", "/", true},
		{"emptyVsSlash", "", "/", false},
		{"different", "/cal/a", "/cal/b", false},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := sameCollectionPath(tc.a, tc.b); got != tc.expected {
				t.Fatalf("sameCollectionPath(%q,%q)=%v, want %v", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}

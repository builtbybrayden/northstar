package finance

import (
	"testing"
)

func TestParseThresholds(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{`[50,75,90,100]`, []int{50, 75, 90, 100}},
		{`[100, 50, 75, 90]`, []int{50, 75, 90, 100}}, // sorted
		{``, []int{}},
		{`50`, []int{50}},
		{`[ 25 , 50 ]`, []int{25, 50}},
	}
	for _, c := range cases {
		got := parseThresholds(c.in)
		if !sliceEq(got, c.want) {
			t.Errorf("parseThresholds(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDollarsFormatting(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0,        "0.00"},
		{99,       "0.99"},
		{100,      "1.00"},
		{74500,    "745.00"},
		{1489200,  "14,892.00"},
		{2847_12,  "2,847.12"},
		{20047200, "200,472.00"},
		{-12345,   "123.45"},          // negative collapses to abs
	}
	for _, c := range cases {
		got := dollars(c.in)
		if got != c.want {
			t.Errorf("dollars(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestThresholdBodyTransitions(t *testing.T) {
	cases := []struct {
		pct           int
		spent, budget int64
		wantContains  string
	}{
		{50, 250_00, 500_00, "Halfway"},
		{75, 375_00, 500_00, "Three-quarters"},
		{90, 450_00, 500_00, "$50.00 left"},
		{100, 612_00, 500_00, "Over budget by $112.00"},
	}
	for _, c := range cases {
		got := thresholdBody(c.pct, c.spent, c.budget)
		if !containsSubstr(got, c.wantContains) {
			t.Errorf("thresholdBody(%d) = %q, want contains %q", c.pct, got, c.wantContains)
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

func sliceEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsSubstr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

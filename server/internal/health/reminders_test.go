package health

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantTimes   []string
		wantDays    []string
	}{
		{"empty", ``, nil, nil},
		{"object_form", `{"times":["07:00","19:00"]}`, []string{"07:00", "19:00"}, nil},
		{"object_with_days", `{"times":["07:00"],"days":["mon","fri"]}`, []string{"07:00"}, []string{"mon", "fri"}},
		{"bare_array", `["08:30","20:00"]`, []string{"08:30", "20:00"}, nil},
		{"garbage", `{not json`, nil, nil},
		{"days_uppercased_normalized_lower", `{"times":["07:00"],"days":["MON"," wed "]}`, []string{"07:00"}, []string{"mon", "wed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseSchedule(c.raw)
			if !slicesEq(got.Times, c.wantTimes) {
				t.Errorf("times = %v, want %v", got.Times, c.wantTimes)
			}
			if !slicesEq(got.Days, c.wantDays) {
				t.Errorf("days = %v, want %v", got.Days, c.wantDays)
			}
		})
	}
}

func TestFiresAt(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		name     string
		sched    SupplementSchedule
		at       time.Time
		wantOK   bool
		wantTime string
	}{
		{
			name:     "match_no_day_filter",
			sched:    SupplementSchedule{Times: []string{"07:00", "19:00"}},
			at:       time.Date(2026, 5, 12, 7, 0, 0, 0, utc),
			wantOK:   true, wantTime: "07:00",
		},
		{
			name:   "miss_wrong_minute",
			sched:  SupplementSchedule{Times: []string{"07:00"}},
			at:     time.Date(2026, 5, 12, 7, 1, 0, 0, utc),
			wantOK: false,
		},
		{
			name:     "match_with_day_filter",
			sched:    SupplementSchedule{Times: []string{"07:00"}, Days: []string{"tue"}},
			at:       time.Date(2026, 5, 12, 7, 0, 0, 0, utc), // 2026-05-12 is a Tuesday
			wantOK:   true, wantTime: "07:00",
		},
		{
			name:   "miss_wrong_day",
			sched:  SupplementSchedule{Times: []string{"07:00"}, Days: []string{"wed"}},
			at:     time.Date(2026, 5, 12, 7, 0, 0, 0, utc),
			wantOK: false,
		},
		{
			name:   "empty_schedule_never_fires",
			sched:  SupplementSchedule{},
			at:     time.Date(2026, 5, 12, 7, 0, 0, 0, utc),
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := c.sched.FiresAt(c.at)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && got != c.wantTime {
				t.Errorf("matched = %q, want %q", got, c.wantTime)
			}
		})
	}
}

func TestInCycleOn(t *testing.T) {
	anchor := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name              string
		now               time.Time
		on, off           int
		want              bool
	}{
		{"no_cycle_always_on", anchor.AddDate(0, 0, 5), 0, 0, true},
		{"day_0_in_window", anchor, 4, 2, true},
		{"day_3_still_on", anchor.AddDate(0, 0, 3), 4, 2, true},
		{"day_4_off", anchor.AddDate(0, 0, 4), 4, 2, false},
		{"day_5_off", anchor.AddDate(0, 0, 5), 4, 2, false},
		{"day_6_back_on", anchor.AddDate(0, 0, 6), 4, 2, true},
		{"future_anchor_treats_on", anchor.AddDate(0, 0, -1), 4, 2, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := InCycleOn(anchor, c.now, c.on, c.off)
			if got != c.want {
				t.Errorf("InCycleOn = %v, want %v", got, c.want)
			}
		})
	}
}

func slicesEq(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
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

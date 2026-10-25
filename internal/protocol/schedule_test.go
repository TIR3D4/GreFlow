package protocol

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestMaintenanceTimezoneOvernightBoundaries(t *testing.T) {
	s := Schedule{Timezone: "Asia/Tehran", MaintenanceWindows: []MaintenanceWindow{{Days: []int{6}, StartMinute: 23 * 60, EndMinute: 60}}}
	for _, tc := range []struct {
		at   string
		want bool
	}{
		{"2026-10-24T22:59:00+03:30", false},
		{"2026-10-24T23:00:00+03:30", true},
		{"2026-10-25T00:59:00+03:30", true},
		{"2026-10-25T01:00:00+03:30", false},
		{"2026-10-26T00:30:00+03:30", false},
	} {
		at, _ := time.Parse(time.RFC3339, tc.at)
		got, e := s.InMaintenance(at)
		if e != nil || got != tc.want {
			t.Fatalf("%s: %v %v", tc.at, got, e)
		}
	}
	s.MaintenanceWindows[0].Days = []int{7}
	if s.ValidateWindows() == nil {
		t.Fatal("accepted invalid weekday")
	}
}

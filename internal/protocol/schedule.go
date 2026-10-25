package protocol

import (
	"fmt"
	"time"
)

// Days uses time.Weekday: Sunday=0. Overnight windows belong to the start day.
type MaintenanceWindow struct {
	Days        []int `json:"days"`
	StartMinute int   `json:"start_minute"`
	EndMinute   int   `json:"end_minute"`
}

func (s Schedule) ValidateWindows() error {
	if len(s.MaintenanceWindows) > 14 {
		return fmt.Errorf("at most 14 maintenance windows")
	}
	for _, w := range s.MaintenanceWindows {
		if len(w.Days) == 0 || len(w.Days) > 7 || w.StartMinute < 0 || w.StartMinute > 1439 || w.EndMinute < 0 || w.EndMinute > 1439 || w.StartMinute == w.EndMinute {
			return fmt.Errorf("invalid maintenance window")
		}
		for _, d := range w.Days {
			if d < 0 || d > 6 {
				return fmt.Errorf("invalid weekday")
			}
		}
	}
	return nil
}
func (s Schedule) InMaintenance(now time.Time) (bool, error) {
	if s.Maintenance {
		return true, nil
	}
	if e := s.ValidateWindows(); e != nil {
		return false, e
	}
	loc, e := time.LoadLocation(s.Timezone)
	if e != nil {
		return false, e
	}
	local := now.In(loc)
	minute := local.Hour()*60 + local.Minute()
	day := int(local.Weekday())
	for _, w := range s.MaintenanceWindows {
		for _, d := range w.Days {
			if w.StartMinute < w.EndMinute {
				if d == day && minute >= w.StartMinute && minute < w.EndMinute {
					return true, nil
				}
			} else if (day == d && minute >= w.StartMinute) || (day == (d+1)%7 && minute < w.EndMinute) {
				return true, nil
			}
		}
	}
	return false, nil
}

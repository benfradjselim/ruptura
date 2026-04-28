package context

import "time"

// Profile is weekday or weekend.
type Profile int

const (
    Weekday Profile = iota
    Weekend
)

type DayOfWeekManager struct{}

func NewDayOfWeekManager() *DayOfWeekManager { return &DayOfWeekManager{} }
func (m *DayOfWeekManager) ProfileOf(t time.Time) Profile {
    if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
        return Weekend
    }
    return Weekday
}
func (m *DayOfWeekManager) IsWeekend(t time.Time) bool {
    return m.ProfileOf(t) == Weekend
}

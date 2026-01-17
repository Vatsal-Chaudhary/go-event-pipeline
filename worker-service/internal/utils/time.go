package utils

import "time"

// rounds timestamp to nearest hour
func GetHourBucket(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// returns current hour bucket
func GetCurrentHour() time.Time {
	return GetHourBucket(time.Now())
}

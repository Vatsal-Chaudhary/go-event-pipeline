package utils

import (
	"testing"
	"time"
)

func TestGetHourBucket(t *testing.T) {
	loc := time.FixedZone("IST", 5*60*60+30*60)
	input := time.Date(2026, time.February, 28, 19, 47, 33, 123456789, loc)

	got := GetHourBucket(input)
	want := time.Date(2026, time.February, 28, 19, 0, 0, 0, loc)

	if !got.Equal(want) {
		t.Fatalf("GetHourBucket() = %v, want %v", got, want)
	}
	if got.Location() != loc {
		t.Fatalf("GetHourBucket() location = %v, want %v", got.Location(), loc)
	}
}

func TestGetCurrentHour(t *testing.T) {
	got := GetCurrentHour()

	if got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
		t.Fatalf("GetCurrentHour() = %v, expected minute/second/nanosecond to be zero", got)
	}
}

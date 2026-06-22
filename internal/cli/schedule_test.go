// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// Reuses freightFixture from freight_test.go (same package): a green-font MUK
// cell (s1, white bg), a green-BACKGROUND MGS cell (s2) that must be excluded,
// and a red-font AIT cell (s3, white bg) that is a scheduled flight.
func TestParseScheduleEntries(t *testing.T) {
	got, err := parseScheduleEntries(freightFixture)
	if err != nil {
		t.Fatalf("parseScheduleEntries: %v", err)
	}
	// MUK (freight) + AIT (scheduled); MGS is green-background → not a flight.
	if len(got) != 2 {
		t.Fatalf("expected 2 flights (MGS excluded by colored bg), got %d: %+v", len(got), got)
	}
	byRoute := map[string]scheduleEntry{}
	for _, e := range got {
		byRoute[e.Route] = e
	}
	muk, ok := byRoute["MUK"]
	if !ok {
		t.Fatalf("MUK flight missing: %+v", got)
	}
	if muk.Category != "freight" {
		t.Errorf("MUK category = %q, want freight", muk.Category)
	}
	if muk.Day != "THU" || muk.Date != "2026-06-18" || muk.Time != "1100" {
		t.Errorf("MUK = %+v, want THU/2026-06-18/1100", muk)
	}
	ait, ok := byRoute["AIT"]
	if !ok {
		t.Fatalf("AIT flight missing: %+v", got)
	}
	if ait.Category != "scheduled" {
		t.Errorf("AIT category = %q, want scheduled (red font is not freight)", ait.Category)
	}
	if _, leaked := byRoute["MGS"]; leaked {
		t.Errorf("MGS (green background, not green font) must not be emitted as a flight")
	}
}

func TestIsRouteText(t *testing.T) {
	good := []string{"MUK", "AIU-MUK", "RAR-PPT", "AIU/NOAA", "MUK-MOI"}
	bad := []string{"", "MON", "THUR", "WEEK", "TO", "SMW", "EFS", "0800", "T"}
	for _, s := range good {
		if !isRouteText(s) {
			t.Errorf("isRouteText(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if isRouteText(s) {
			t.Errorf("isRouteText(%q) = true, want false", s)
		}
	}
}

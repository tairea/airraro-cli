// Copyright 2026 Ian Tairea and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// freightFixture is a minimal published-sheet shape: a style block with one
// green-FONT class (s1), one green-BACKGROUND class that must NOT count (s2),
// and a non-green class (s3); plus a one-block grid exercising colspan offset
// (so the green cell lands on the THUR column) and a trailing rowspan.
const freightFixture = `
<style>
.s1{background-color:#ffffff;color:#00b050;font-size:8pt;}
.s2{background-color:#00b050;color:#000000;}
.s3{background-color:#ffffff;color:#ff0000;}
</style>
<table>
<tr><td class="s3">25</td><td>MON</td><td>TUE</td><td>WED</td><td>THUR</td><td>FRI</td><td>SAT</td><td>SUN</td><td rowspan="3">&nbsp;</td></tr>
<tr><td>25</td><td>6/15/2026</td><td>16Jun</td><td>17Jun</td><td>18Jun</td><td>19Jun</td><td>20Jun</td><td>21Jun</td></tr>
<tr><td>1100</td><td colspan="3"></td><td class="s1">MUK</td><td class="s2">MGS</td><td class="s3">AIT</td><td></td></tr>
</table>`

func TestGreenFontClasses(t *testing.T) {
	green := greenFontClasses(freightFixture)
	if !green["s1"] {
		t.Errorf("s1 (green font) should be detected")
	}
	if green["s2"] {
		t.Errorf("s2 is green BACKGROUND, not font; must not count as freight")
	}
	if green["s3"] {
		t.Errorf("s3 (red font) must not count")
	}
}

func TestParseFreightRuns(t *testing.T) {
	runs, err := parseFreightRuns(freightFixture)
	if err != nil {
		t.Fatalf("parseFreightRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 green-font freight run, got %d: %+v", len(runs), runs)
	}
	r := runs[0]
	if r.Route != "MUK" {
		t.Errorf("route = %q, want MUK", r.Route)
	}
	if r.Week != 25 {
		t.Errorf("week = %d, want 25", r.Week)
	}
	if r.Date != "2026-06-18" {
		t.Errorf("date = %q, want 2026-06-18 (colspan must offset green cell to the THUR column)", r.Date)
	}
	if r.Day != "THU" {
		t.Errorf("day = %q, want THU", r.Day)
	}
	if r.Time != "1100" {
		t.Errorf("time = %q, want 1100", r.Time)
	}
	if !r.Freight {
		t.Errorf("freight flag should be true")
	}
	if len(r.Islands) != 1 || r.Islands[0] != "MUK" {
		t.Errorf("islands = %v, want [MUK]", r.Islands)
	}
}

func TestParseFreightDateShapes(t *testing.T) {
	cases := map[string]string{
		"16Jun":     "2026-06-16",
		"01Jul":     "2026-07-01",
		"6/15/2026": "2026-06-15",
		"":          "",
		"AIT":       "",
	}
	for in, want := range cases {
		if got := parseGridDate(in, 2026); got != want {
			t.Errorf("parseGridDate(%q) = %q, want %q", in, got, want)
		}
	}
}

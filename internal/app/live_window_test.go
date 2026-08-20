package app

import "testing"

func TestLogicalSizeFromPhysical(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		pw, ph     int
		scale      float64
		wantW, wantH int
	}{
		{name: "1x", pw: 1100, ph: 760, scale: 1, wantW: 1100, wantH: 760},
		{name: "2x retina", pw: 2200, ph: 1520, scale: 2, wantW: 1100, wantH: 760},
		{name: "zero scale falls back", pw: 800, ph: 600, scale: 0, wantW: 800, wantH: 600},
		{name: "truncates like Context.Size", pw: 1001, ph: 801, scale: 2, wantW: 500, wantH: 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := logicalSizeFromPhysical(tc.pw, tc.ph, tc.scale)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("logicalSizeFromPhysical(%d,%d,%v) = %dx%d, want %dx%d",
					tc.pw, tc.ph, tc.scale, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

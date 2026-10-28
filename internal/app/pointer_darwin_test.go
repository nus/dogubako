//go:build darwin

package app

import "testing"

func TestMouseInContentRect(t *testing.T) {
	r := nsRect{Origin: nsPoint{X: 10, Y: 20}, Size: nsSize{Width: 100, Height: 50}}
	tests := []struct {
		name string
		p    nsPoint
		want bool
	}{
		{name: "inside", p: nsPoint{X: 50, Y: 40}, want: true},
		{name: "min x", p: nsPoint{X: 10, Y: 40}, want: true},
		{name: "max x", p: nsPoint{X: 110, Y: 40}, want: false},
		{name: "min y", p: nsPoint{X: 50, Y: 20}, want: false},
		{name: "max y", p: nsPoint{X: 50, Y: 70}, want: true},
		{name: "just above min y", p: nsPoint{X: 50, Y: 20.1}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mouseInContentRect(tt.p, r); got != tt.want {
				t.Fatalf("mouseInContentRect(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

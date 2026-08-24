package app

import (
	"testing"
	"time"
)

func TestFormatStopwatch(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00:00.00"},
		{-time.Second, "00:00:00.00"},
		{10 * time.Millisecond, "00:00:00.01"},
		{1234 * time.Millisecond, "00:00:01.23"},
		{61*time.Second + 40*time.Millisecond, "00:01:01.04"},
		{time.Hour + 2*time.Minute + 3*time.Second + 40*time.Millisecond, "01:02:03.04"},
		{10*time.Hour + 11*time.Minute + 12*time.Second + 990*time.Millisecond, "10:11:12.99"},
	}
	for _, c := range cases {
		if got := FormatStopwatch(c.d); got != c.want {
			t.Errorf("FormatStopwatch(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestStopwatchToggleAndReset(t *testing.T) {
	var m StopwatchModel
	t0 := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if m.Running() || m.CanReset() {
		t.Fatal("fresh watch should be stopped and empty")
	}
	if got := FormatStopwatch(m.elapsedAt(t0)); got != "00:00:00.00" {
		t.Fatalf("fresh display = %q", got)
	}

	m.toggleAt(t0)
	if !m.Running() {
		t.Fatal("expected running after start")
	}
	t1 := t0.Add(1530 * time.Millisecond)
	if got := FormatStopwatch(m.elapsedAt(t1)); got != "00:00:01.53" {
		t.Fatalf("running display = %q", got)
	}

	m.toggleAt(t1)
	if m.Running() {
		t.Fatal("expected paused")
	}
	if !m.CanReset() {
		t.Fatal("paused watch with elapsed time should reset")
	}
	t2 := t1.Add(5 * time.Second)
	if got := FormatStopwatch(m.elapsedAt(t2)); got != "00:00:01.53" {
		t.Fatalf("paused display should freeze: %q", got)
	}

	m.toggleAt(t2)
	t3 := t2.Add(470 * time.Millisecond)
	if got := FormatStopwatch(m.elapsedAt(t3)); got != "00:00:02.00" {
		t.Fatalf("resumed display = %q", got)
	}

	gen := m.Generation()
	m.Reset()
	if m.Running() || m.CanReset() {
		t.Fatal("reset should stop and clear")
	}
	if m.Generation() == gen {
		t.Fatal("reset should bump generation")
	}
	if got := FormatStopwatch(m.elapsedAt(t3.Add(time.Hour))); got != "00:00:00.00" {
		t.Fatalf("after reset = %q", got)
	}

	m.Reset()
	if m.Generation() == 0 {
		t.Fatal("generation should stay bumped after first reset")
	}
}

func TestStopwatchDisplayTicks(t *testing.T) {
	var m StopwatchModel
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.toggleAt(t0)
	m.toggleAt(t0.Add(1234 * time.Millisecond))
	if got := m.DisplayTicks(); got != 123 {
		t.Fatalf("ticks = %d, want 123", got)
	}
}

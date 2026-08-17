//go:build !linux

package memstat

func fillRSS(*Snapshot) {}

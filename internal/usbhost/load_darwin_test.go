//go:build darwin

package usbhost

import "testing"

func TestIOReturnNameAborted(t *testing.T) {
	if got := ioReturnName(ioReturnAborted); got != "kIOReturnAborted" {
		t.Fatalf("ioReturnName(aborted) = %q", got)
	}
	err := ioReturnErr(ioReturnAborted)
	if err == nil || err.Error() != "Unable to send IO. (0xe00002eb kIOReturnAborted)" {
		t.Fatalf("ioReturnErr = %v", err)
	}
}

func TestShouldClearStallCode(t *testing.T) {
	if shouldClearStallCode(ioReturnAborted) {
		t.Fatal("abort is not a halt")
	}
	if shouldClearStallCode(ioReturnTimeout) {
		t.Fatal("timeout is not a halt")
	}
	if !shouldClearStallCode(ioUSBPipeStalled) {
		t.Fatal("stalled pipe should be cleared")
	}
	if !shouldClearStallCode(ioReturnNotResponding) {
		t.Fatal("not-responding is an IO error on the pipe")
	}
}

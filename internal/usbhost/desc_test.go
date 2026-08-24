package usbhost

import "testing"

func TestIsMTPInterface(t *testing.T) {
	if !isMTPInterface(0x18d1, 6, 1, 1) {
		t.Fatal("still-image PTP/MTP")
	}
	if !isMTPInterface(0x18d1, 0xff, 0xff, 0) {
		t.Fatal("android vendor MTP")
	}
	if isMTPInterface(0x05ac, 6, 1, 1) {
		t.Fatal("apple PTP should be skipped")
	}
	if isMTPInterface(0x18d1, 0xff, 0x42, 1) {
		t.Fatal("ADB is not MTP")
	}
}

func TestParseEndpoints(t *testing.T) {
	// Config + one still-image interface with bulk-out 0x01, bulk-in 0x81, interrupt 0x82.
	config := []byte{
		9, 2, 39, 0, 1, 1, 0, 0x80, 50,
		9, 4, 0, 0, 3, 6, 1, 1, 0,
		7, 5, 0x01, 2, 0x00, 0x02, 0,
		7, 5, 0x81, 2, 0x00, 0x02, 0,
		7, 5, 0x82, 3, 0x40, 0x00, 8,
	}
	ep, ok := parseEndpoints(config, 0, 0)
	if !ok {
		t.Fatal("expected endpoints")
	}
	if ep.bulkOut != 0x01 || ep.bulkIn != 0x81 || ep.interruptIn != 0x82 {
		t.Fatalf("eps = %+v", ep)
	}
	if ep.maxPacketOut != 512 {
		t.Fatalf("mps = %d", ep.maxPacketOut)
	}
}

func TestParseEndpointsMissingBulk(t *testing.T) {
	config := []byte{
		9, 2, 18, 0, 1, 1, 0, 0x80, 50,
		9, 4, 0, 0, 0, 6, 1, 1, 0,
	}
	if _, ok := parseEndpoints(config, 0, 0); ok {
		t.Fatal("should fail without bulk endpoints")
	}
}

func TestListDoesNotPanic(t *testing.T) {
	infos, err := List()
	if err == ErrUnsupported {
		t.Skip("USB host not available")
	}
	if err != nil {
		t.Logf("list: %v", err)
		return
	}
	t.Logf("mtp interfaces: %d", len(infos))
	for _, info := range infos {
		t.Logf("  %s %s", info.ID, info.Label())
	}
}

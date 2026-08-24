package usbhost

const (
	usbDTConfig    = 2
	usbDTInterface = 4
	usbDTEndpoint  = 5

	usbClassStillImage = 6
	usbClassVendor     = 0xff

	usbXferBulk = 2
	usbXferInt  = 3
	usbDirIn    = 0x80

	appleVendorID = 0x05ac
)

type endpoints struct {
	bulkOut      int
	bulkIn       int
	interruptIn  int
	maxPacketOut int
}

func isMTPInterface(vendorID, class, sub, proto int) bool {
	if vendorID == appleVendorID {
		return false
	}
	if class == usbClassStillImage && proto == 1 {
		return true
	}
	// Android's always-on MTP interface.
	if class == usbClassVendor && sub == usbClassVendor && proto == 0 {
		return true
	}
	return false
}

func parseEndpoints(config []byte, ifaceNum, alt int) (endpoints, bool) {
	var ep endpoints
	if len(config) < 9 || config[1] != usbDTConfig {
		return ep, false
	}
	total := int(config[2]) | int(config[3])<<8
	if total > len(config) {
		total = len(config)
	}
	inIface := false
	for i := 0; i+2 <= total; {
		n := int(config[i])
		if n < 2 || i+n > total {
			break
		}
		typ := config[i+1]
		switch typ {
		case usbDTInterface:
			if n < 9 {
				break
			}
			num := int(config[i+2])
			al := int(config[i+3])
			inIface = num == ifaceNum && al == alt
		case usbDTEndpoint:
			if !inIface || n < 7 {
				break
			}
			addr := int(config[i+2])
			xfer := int(config[i+3] & 0x03)
			mps := int(config[i+4]) | int(config[i+5])<<8
			mps &= 0x07ff
			in := addr&usbDirIn != 0
			switch {
			case xfer == usbXferBulk && !in:
				ep.bulkOut = addr
				ep.maxPacketOut = mps
			case xfer == usbXferBulk && in:
				ep.bulkIn = addr
			case xfer == usbXferInt && in:
				ep.interruptIn = addr
			}
		}
		i += n
	}
	if ep.maxPacketOut <= 0 {
		ep.maxPacketOut = 512
	}
	return ep, ep.bulkOut != 0 && ep.bulkIn != 0
}

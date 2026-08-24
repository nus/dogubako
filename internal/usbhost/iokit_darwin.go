//go:build darwin

package usbhost

import (
	"fmt"
	"strconv"

	"github.com/ebitengine/purego/objc"
)

func list() ([]Info, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	svcs, err := matchingServices()
	if err != nil {
		return nil, err
	}
	defer releaseServices(svcs)

	var out []Info
	for _, svc := range svcs {
		info, ok := infoFromService(svc)
		if !ok {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

func matchingServices() ([]uint32, error) {
	matching := ioServiceMatching("IOUSBHostInterface")
	if matching == 0 {
		return nil, fmt.Errorf("IOServiceMatching failed")
	}
	var iter uint32
	if kr := ioServiceGetMatchingServices(0, matching, &iter); kr != kernSuccess {
		return nil, fmt.Errorf("IOServiceGetMatchingServices: 0x%x", uint32(kr))
	}
	defer ioObjectRelease(iter)

	var svcs []uint32
	for {
		svc := ioIteratorNext(iter)
		if svc == 0 {
			break
		}
		svcs = append(svcs, svc)
	}
	return svcs, nil
}

func releaseServices(svcs []uint32) {
	for _, s := range svcs {
		ioObjectRelease(s)
	}
}

func infoFromService(svc uint32) (Info, bool) {
	class := intProp(svc, "bInterfaceClass")
	sub := intProp(svc, "bInterfaceSubClass")
	proto := intProp(svc, "bInterfaceProtocol")
	vid := intProp(svc, "idVendor")
	if !isMTPInterface(vid, class, sub, proto) {
		return Info{}, false
	}
	info := Info{
		VendorID:     vid,
		ProductID:    intProp(svc, "idProduct"),
		LocationID:   uint32(intProp(svc, "locationID")),
		Serial:       strProp(svc, "USB Serial Number"),
		Product:      strProp(svc, "USB Product Name"),
		Manufacturer: strProp(svc, "USB Vendor Name"),
		Interface:    intProp(svc, "bInterfaceNumber"),
	}
	if info.Product == "" {
		info.Product = "MTP device"
	}
	info.ID = deviceID(info)
	return info, true
}

func deviceID(info Info) string {
	ser := info.Serial
	if ser == "" {
		ser = strconv.FormatUint(uint64(info.LocationID), 16)
	}
	return fmt.Sprintf("%04x:%04x:%s:%d", info.VendorID, info.ProductID, ser, info.Interface)
}

func intProp(svc uint32, key string) int {
	cf := copyProp(svc, key)
	if cf == 0 {
		return 0
	}
	defer cfRelease(cf)
	id := objc.ID(cf)
	if class_NSNumber != 0 && id.Send(sel_isKindOfClass, class_NSNumber) != 0 {
		return int(objc.Send[int64](id, sel_integerValue))
	}
	return 0
}

func strProp(svc uint32, key string) string {
	cf := copyProp(svc, key)
	if cf == 0 {
		return ""
	}
	defer cfRelease(cf)
	id := objc.ID(cf)
	if class_NSString != 0 && id.Send(sel_isKindOfClass, class_NSString) != 0 {
		return goString(id)
	}
	return ""
}

func copyProp(svc uint32, key string) uintptr {
	k := nsString(key)
	if k == 0 {
		return 0
	}
	defer k.Send(sel_release)
	return ioRegistryEntryCreateCFProperty(svc, uintptr(k), 0, 0)
}

func findService(id string) (uint32, Info, error) {
	svcs, err := matchingServices()
	if err != nil {
		return 0, Info{}, err
	}
	defer func() {
		for _, s := range svcs {
			if s != 0 {
				ioObjectRelease(s)
			}
		}
	}()
	for i, svc := range svcs {
		info, ok := infoFromService(svc)
		if !ok || info.ID != id {
			continue
		}
		svcs[i] = 0
		return svc, info, nil
	}
	return 0, Info{}, ErrNotFound
}

//go:build darwin

package usbhost

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	initOptionsNone = 0
	// IOUSBHostObjectInitOptionsDeviceSeize. Asks the current owner
	// (Image Capture / PTPCamera) to close via kUSBHostMessageDeviceIsRequestingClose.
	initOptionsSeize = 1 << 1

	kernSuccess = 0

	ioReturnNoMemory        = 0xe00002bd
	ioReturnNoResources     = 0xe00002be
	ioReturnNoDevice        = 0xe00002c0
	ioReturnNotPrivileged   = 0xe00002c1
	ioReturnBadArgument     = 0xe00002c2
	ioReturnExclusiveAccess = 0xe00002c5
	ioReturnUnsupported     = 0xe00002c7
	ioReturnNotReady        = 0xe00002d5
	ioReturnTimeout         = 0xe00002d6
	ioReturnNotAttached     = 0xe00002e0
	ioReturnAborted         = 0xe00002eb
	ioReturnNotResponding   = 0xe00002ed
	ioUSBPipeStalled        = 0xe0004061

	// kUSBHostMessageDeviceIsRequestingClose (IOUSBHostFamilyDefinitions.h)
	usbHostMessageDeviceIsRequestingClose = 0xe0005003
)

// ioReturnName maps the IOReturn codes IOUSBHost reports for bulk transfers.
// The framework's localized descriptions ("Unable to send IO.") are identical
// for every failure cause, so the numeric code is what identifies the problem.
func ioReturnName(code uint32) string {
	switch code {
	case ioReturnNoMemory:
		return "kIOReturnNoMemory"
	case ioReturnNoResources:
		return "kIOReturnNoResources"
	case ioReturnNoDevice:
		return "kIOReturnNoDevice"
	case ioReturnNotPrivileged:
		return "kIOReturnNotPrivileged"
	case ioReturnBadArgument:
		return "kIOReturnBadArgument"
	case ioReturnExclusiveAccess:
		return "kIOReturnExclusiveAccess"
	case ioReturnUnsupported:
		return "kIOReturnUnsupported"
	case ioReturnNotReady:
		return "kIOReturnNotReady"
	case ioReturnTimeout:
		return "kIOReturnTimeout"
	case ioReturnNotAttached:
		return "kIOReturnNotAttached"
	case ioReturnAborted:
		return "kIOReturnAborted"
	case ioReturnNotResponding:
		return "kIOReturnNotResponding"
	case ioUSBPipeStalled:
		return "kIOUSBPipeStalled"
	default:
		return ""
	}
}

var (
	loadOnce sync.Once
	loadErr  error

	ioServiceMatching               func(name string) uintptr
	ioServiceGetMatchingServices    func(port uint32, matching uintptr, iter *uint32) int32
	ioIteratorNext                  func(iter uint32) uint32
	ioObjectRelease                 func(obj uint32) int32
	ioRegistryEntryCreateCFProperty func(entry uint32, key uintptr, allocator uintptr, options uint32) uintptr
	cfRelease                       func(cf uintptr)

	class_IOUSBHostInterface objc.Class
	class_NSMutableData      objc.Class
	class_NSString           objc.Class
	class_NSAutoreleasePool  objc.Class
	class_NSNumber           objc.Class

	sel_alloc                objc.SEL
	sel_init                 objc.SEL
	sel_release              objc.SEL
	sel_retain               objc.SEL
	sel_drain                objc.SEL
	sel_new                  objc.SEL
	sel_initWithUTF8String   objc.SEL
	sel_intValue             objc.SEL
	sel_integerValue         objc.SEL
	sel_UTF8String           objc.SEL
	sel_isKindOfClass        objc.SEL
	sel_localizedDescription objc.SEL
	sel_code                 objc.SEL
	sel_domain               objc.SEL
	sel_initWithIOService    objc.SEL
	sel_destroy              objc.SEL
	sel_copyPipe             objc.SEL
	sel_configurationDesc    objc.SEL
	sel_interfaceDesc        objc.SEL
	sel_enqueueIORequest     objc.SEL
	sel_abort                objc.SEL
	sel_clearStall           objc.SEL
	sel_setIdleTimeout       objc.SEL
	sel_idleTimeout          objc.SEL
	sel_ioDataWithCapacity   objc.SEL
	sel_initWithLength       objc.SEL
	sel_initWithBytes        objc.SEL
	sel_mutableBytes         objc.SEL
	sel_length               objc.SEL
	sel_setLength            objc.SEL
)

func ensureLoaded() error {
	loadOnce.Do(func() {
		loadErr = loadFrameworks()
	})
	return loadErr
}

func loadFrameworks() error {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
		return fmt.Errorf("Foundation: %w", err)
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
		return fmt.Errorf("CoreFoundation: %w", err)
	}
	iokit, err := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("IOKit: %w", err)
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/IOUSBHost.framework/IOUSBHost", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
		return fmt.Errorf("IOUSBHost: %w", err)
	}
	purego.RegisterLibFunc(&ioServiceMatching, iokit, "IOServiceMatching")
	purego.RegisterLibFunc(&ioServiceGetMatchingServices, iokit, "IOServiceGetMatchingServices")
	purego.RegisterLibFunc(&ioIteratorNext, iokit, "IOIteratorNext")
	purego.RegisterLibFunc(&ioObjectRelease, iokit, "IOObjectRelease")
	purego.RegisterLibFunc(&ioRegistryEntryCreateCFProperty, iokit, "IORegistryEntryCreateCFProperty")
	purego.RegisterLibFunc(&cfRelease, purego.RTLD_DEFAULT, "CFRelease")

	class_IOUSBHostInterface = objc.GetClass("IOUSBHostInterface")
	class_NSMutableData = objc.GetClass("NSMutableData")
	class_NSString = objc.GetClass("NSString")
	class_NSAutoreleasePool = objc.GetClass("NSAutoreleasePool")
	class_NSNumber = objc.GetClass("NSNumber")
	if class_IOUSBHostInterface == 0 || class_NSMutableData == 0 || class_NSString == 0 {
		return fmt.Errorf("IOUSBHost Objective-C classes not found")
	}

	sel_alloc = objc.RegisterName("alloc")
	sel_init = objc.RegisterName("init")
	sel_release = objc.RegisterName("release")
	sel_retain = objc.RegisterName("retain")
	sel_drain = objc.RegisterName("drain")
	sel_new = objc.RegisterName("new")
	sel_initWithUTF8String = objc.RegisterName("initWithUTF8String:")
	sel_intValue = objc.RegisterName("intValue")
	sel_integerValue = objc.RegisterName("integerValue")
	sel_UTF8String = objc.RegisterName("UTF8String")
	sel_isKindOfClass = objc.RegisterName("isKindOfClass:")
	sel_localizedDescription = objc.RegisterName("localizedDescription")
	sel_code = objc.RegisterName("code")
	sel_domain = objc.RegisterName("domain")
	sel_initWithIOService = objc.RegisterName("initWithIOService:options:queue:error:interestHandler:")
	sel_destroy = objc.RegisterName("destroy")
	sel_copyPipe = objc.RegisterName("copyPipeWithAddress:error:")
	sel_configurationDesc = objc.RegisterName("configurationDescriptor")
	sel_interfaceDesc = objc.RegisterName("interfaceDescriptor")
	sel_enqueueIORequest = objc.RegisterName("enqueueIORequestWithData:completionTimeout:error:completionHandler:")
	sel_abort = objc.RegisterName("abortWithError:")
	sel_clearStall = objc.RegisterName("clearStallWithError:")
	sel_setIdleTimeout = objc.RegisterName("setIdleTimeout:error:")
	sel_idleTimeout = objc.RegisterName("idleTimeout")
	sel_ioDataWithCapacity = objc.RegisterName("ioDataWithCapacity:error:")
	sel_initWithLength = objc.RegisterName("initWithLength:")
	sel_initWithBytes = objc.RegisterName("initWithBytes:length:")
	sel_mutableBytes = objc.RegisterName("mutableBytes")
	sel_length = objc.RegisterName("length")
	sel_setLength = objc.RegisterName("setLength:")
	return nil
}

func nsString(s string) objc.ID {
	return objc.ID(class_NSString).Send(sel_alloc).Send(sel_initWithUTF8String, s)
}

func goString(id objc.ID) string {
	if id == 0 {
		return ""
	}
	p := objc.Send[unsafe.Pointer](id, sel_UTF8String)
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 {
		n++
	}
	if n == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(p), n))
}

func nsError(errID objc.ID) error {
	if errID == 0 {
		return fmt.Errorf("IOUSBHost error")
	}
	code := nsErrorCode(errID)
	msg := goString(errID.Send(sel_localizedDescription))
	if code == ioReturnExclusiveAccess || code == ioReturnNotPrivileged {
		if msg == "" {
			return ErrBusy
		}
		return fmt.Errorf("%w: %s", ErrBusy, msg)
	}
	detail := fmt.Sprintf("0x%08x", code)
	if name := ioReturnName(code); name != "" {
		detail += " " + name
	}
	if msg == "" {
		return fmt.Errorf("IOUSBHost error %s", detail)
	}
	return fmt.Errorf("%s (%s)", msg, detail)
}

func ioReturnErr(code uint32) error {
	detail := fmt.Sprintf("0x%08x", code)
	if name := ioReturnName(code); name != "" {
		detail += " " + name
	}
	return fmt.Errorf("Unable to send IO. (%s)", detail)
}

func nsErrorCode(errID objc.ID) uint32 {
	if errID == 0 {
		return 0
	}
	return uint32(objc.Send[int64](errID, sel_code))
}

// shouldClearStallCode reports whether the pipe is likely Halted. Timeout and
// abort are host-side cancellations; clearStallWithError: would reset the data
// toggle and desynchronize a bulk stream.
func shouldClearStallCode(code uint32) bool {
	switch code {
	case 0, ioReturnTimeout, ioReturnAborted, ioReturnNoDevice, ioReturnNotAttached:
		return false
	default:
		return true
	}
}

func newPool() objc.ID {
	return objc.ID(class_NSAutoreleasePool).Send(sel_new)
}

//go:build darwin && hidvirtual

package vhid

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
    "github.com/bulwarkid/virtual-fido/ctap_hid"
)

var (
	libobjc     uintptr
	foundation  uintptr
	corehid     uintptr

	// Objective-C Runtime
	objc_alloc              func(uintptr) uintptr
	objc_msgSend            func(uintptr, uintptr, ...any) uintptr
    
    // Typed msgSend for stability on ARM64
    msgSend_none            func(uintptr, uintptr) uintptr
    msgSend_ptr             func(uintptr, uintptr, uintptr) uintptr
    msgSend_ptr_ptr         func(uintptr, uintptr, uintptr, uintptr) uintptr
    msgSend_ptr_uint64_ptr  func(uintptr, uintptr, uintptr, uint64, uintptr) uintptr

	objc_getClass           func(string) uintptr
    object_getClass         func(uintptr) uintptr
    class_getMethodImplementation func(uintptr, uintptr) uintptr
    class_getInstanceMethod func(uintptr, uintptr) uintptr
    class_getClassMethod    func(uintptr, uintptr) uintptr
	sel_registerName        func(string) uintptr
	objc_allocateClassPair  func(uintptr, string, int) uintptr
	class_addMethod         func(uintptr, uintptr, uintptr, string) bool
	objc_registerClassPair  func(uintptr)
    
    // Selectors
    sel_alloc               uintptr
    sel_init                uintptr
    sel_new                 uintptr
    sel_stringWithUTF8String uintptr
    sel_numberWithInt       uintptr
    sel_dictionaryWithObjectsForKeys uintptr
    sel_dataWithBytesLength uintptr
    sel_activate            uintptr
    sel_cancel              uintptr
    sel_setDelegate         uintptr
    sel_dispatchInputReport uintptr
    sel_initWithProperties  uintptr

    // Classes
    cls_NSObject            uintptr
    cls_NSString            uintptr
    cls_NSNumber            uintptr
    cls_NSDictionary        uintptr
    cls_NSData              uintptr
    cls_HIDVirtualDevice    uintptr
)

func init() {
    var err error
    libobjc, err = purego.Dlopen("libobjc.A.dylib", purego.RTLD_GLOBAL)
    if err != nil {
        panic(fmt.Errorf("failed to load libobjc: %w", err))
    }
    foundation, err = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL)
    if err != nil {
         panic(fmt.Errorf("failed to load Foundation: %w", err))
    }
    // Try standard path for CoreHID (macOS 15+ has it in System/Library/Frameworks/CoreHID.framework/CoreHID commonly, or standard linker path if available. But dlopen usually needs full path for frameworks if not in standard cache, though Foundation loaded fine?)
    // Actually, on new macOS, libraries are in the dyld cache. purego.Dlopen("CoreHID.framework/CoreHID") might work if headers allow.
    // CoreHID is relatively new (DriverKit user client). It might be in /System/Library/Frameworks/CoreHID.framework/CoreHID
    corehid, err = purego.Dlopen("/System/Library/Frameworks/CoreHID.framework/CoreHID", purego.RTLD_GLOBAL)
    if err != nil {
        // Fallback or panic. It is required.
        panic(fmt.Errorf("failed to load CoreHID: %w", err))
    }

    purego.RegisterLibFunc(&objc_alloc, libobjc, "objc_alloc")
    purego.RegisterLibFunc(&objc_msgSend, libobjc, "objc_msgSend")
    purego.RegisterLibFunc(&msgSend_none, libobjc, "objc_msgSend")
    purego.RegisterLibFunc(&msgSend_ptr, libobjc, "objc_msgSend")
    purego.RegisterLibFunc(&msgSend_ptr_ptr, libobjc, "objc_msgSend")
    purego.RegisterLibFunc(&msgSend_ptr_uint64_ptr, libobjc, "objc_msgSend")
    
    purego.RegisterLibFunc(&objc_getClass, libobjc, "objc_getClass")
    purego.RegisterLibFunc(&object_getClass, libobjc, "object_getClass")
    purego.RegisterLibFunc(&class_getMethodImplementation, libobjc, "class_getMethodImplementation")
    purego.RegisterLibFunc(&class_getInstanceMethod, libobjc, "class_getInstanceMethod")
    purego.RegisterLibFunc(&class_getClassMethod, libobjc, "class_getClassMethod")
    purego.RegisterLibFunc(&sel_registerName, libobjc, "sel_registerName")
    purego.RegisterLibFunc(&objc_allocateClassPair, libobjc, "objc_allocateClassPair")
    purego.RegisterLibFunc(&class_addMethod, libobjc, "class_addMethod")
    purego.RegisterLibFunc(&objc_registerClassPair, libobjc, "objc_registerClassPair")

    // Logic to fetch selectors lazily or init them here
    sel_alloc = sel_registerName("alloc")
    sel_init = sel_registerName("init")
    sel_new = sel_registerName("new")
    sel_stringWithUTF8String = sel_registerName("stringWithUTF8String:")
    sel_numberWithInt = sel_registerName("numberWithInt:")
    sel_dictionaryWithObjectsForKeys = sel_registerName("dictionaryWithObjects:forKeys:")
    sel_dataWithBytesLength = sel_registerName("dataWithBytes:length:")
    sel_activate = sel_registerName("activate")
    sel_cancel = sel_registerName("cancel")
    sel_setDelegate = sel_registerName("setDelegate:")
    sel_dispatchInputReport = sel_registerName("dispatchInputReport:timestamp:error:")
    sel_initWithProperties = sel_registerName("initWithProperties:")

    cls_NSObject = objc_getClass("NSObject")
    cls_NSString = objc_getClass("NSString")
    cls_NSNumber = objc_getClass("NSNumber")
    cls_NSDictionary = objc_getClass("NSDictionary")
    cls_NSData = objc_getClass("NSData")
    cls_NSBundle := objc_getClass("NSBundle")
    
    // Load CoreHID via NSBundle to ensure classes are registered
    if cls_NSBundle != 0 {
        sel_bundleWithPath := sel_registerName("bundleWithPath:")
        sel_load := sel_registerName("load")
        
        pathStr := nsString("/System/Library/Frameworks/CoreHID.framework")
        bundle := msgSend(cls_NSBundle, sel_bundleWithPath, pathStr)
        if bundle != 0 {
            success := msgSend(bundle, sel_load)
            fmt.Printf("[HID] NSBundle load CoreHID success: %v\n", success != 0)
        } else {
             fmt.Printf("[HID] Failed to get NSBundle for CoreHID\n")
        }
    }

    cls_HIDVirtualDevice = objc_getClass("CoreHID.HIDVirtualDevice")
    if cls_HIDVirtualDevice == 0 {
         // Fallback to non-namespaced just in case
        cls_HIDVirtualDevice = objc_getClass("HIDVirtualDevice")
    }
    
    // Diagnostic: List classes starting with HID
    if cls_HIDVirtualDevice == 0 {
        objc_getClassList := func(buffer uintptr, bufferLen int) int {
            var f func(uintptr, int) int
            purego.RegisterLibFunc(&f, libobjc, "objc_getClassList")
            return f(buffer, bufferLen)
        }
        class_getName := func(cls uintptr) string {
            var f func(uintptr) string
            purego.RegisterLibFunc(&f, libobjc, "class_getName")
            return f(cls)
        }
        
        numClasses := objc_getClassList(0, 0)
        classes := make([]uintptr, numClasses)
        objc_getClassList(uintptr(unsafe.Pointer(&classes[0])), numClasses)
        
        fmt.Printf("[HID] Total classes found: %d. Searching for *HID* or *Virtual* classes...\n", numClasses)
        for _, cls := range classes {
            name := class_getName(cls)
            containsHID := false
            containsVirtual := false
            for i := 0; i <= len(name)-3; i++ {
                if name[i:i+3] == "HID" {
                    containsHID = true
                }
            }
             for i := 0; i <= len(name)-7; i++ {
                if name[i:i+7] == "Virtual" {
                    containsVirtual = true
                }
            }
            if containsHID || containsVirtual {
                fmt.Printf("[HID] Found class: %s\n", name)
            }
        }
    }

    fmt.Printf("[HID] Classes: NSObject=%x, NSString=%x, HIDVirtualDevice=%x\n", cls_NSObject, cls_NSString, cls_HIDVirtualDevice)
}

// Helpers
func msgSend(receiver uintptr, selector uintptr, args ...uintptr) uintptr {
    if receiver == 0 { return 0 }
    cls := object_getClass(receiver)
    method := class_getInstanceMethod(cls, selector)
    if method == 0 {
        // Try class method if instance method fails
        method = class_getClassMethod(cls, selector)
    }
    if method == 0 { return 0 }
    
    // method_invoke(receiver, method, ...args)
    f, _ := purego.Dlsym(libobjc, "method_invoke")
    
    var r uintptr
    switch len(args) {
    case 0:
        r, _, _ = purego.SyscallN(f, receiver, method)
    case 1:
        r, _, _ = purego.SyscallN(f, receiver, method, args[0])
    case 2:
        r, _, _ = purego.SyscallN(f, receiver, method, args[0], args[1])
    case 3:
        r, _, _ = purego.SyscallN(f, receiver, method, args[0], args[1], args[2])
    default:
        fullArgs := make([]uintptr, 2+len(args))
        fullArgs[0] = receiver
        fullArgs[1] = method
        copy(fullArgs[2:], args)
        r, _, _ = purego.SyscallN(f, fullArgs...)
    }
    return r
}

func nsString(s string) uintptr {
    cStr := append([]byte(s), 0)
    return msgSend(cls_NSString, sel_stringWithUTF8String, uintptr(unsafe.Pointer(&cStr[0])))
}

func nsNumber(i int) uintptr {
    return msgSend(cls_NSNumber, sel_numberWithInt, uintptr(i))
}

func nsData(b []byte) uintptr {
    if len(b) == 0 {
        return msgSend(cls_NSData, sel_new)
    }
    return msgSend(cls_NSData, sel_dataWithBytesLength, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}

func bytePtrToString(p *byte) string {
    if p == nil {
        return ""
    }
    var s []byte
    for *p != 0 {
        s = append(s, *p)
        p = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 1))
    }
    return string(s)
}

// Report Descriptor for FIDO
var fidoReportDescriptor = []byte{
    0x06, 0xD0, 0xF1,  // Usage Page (FIDO Alliance)
    0x09, 0x01,        // Usage (U2F Authenticator Device)
    0xA1, 0x01,        // Collection (Application)
    0x09, 0x20,        //   Usage (Input Report Data)
    0x15, 0x00,        //   Logical Minimum (0)
    0x26, 0xFF, 0x00,  //   Logical Maximum (255)
    0x75, 0x08,        //   Report Size (8 bits)
    0x95, 0x40,        //   Report Count (64 bytes)
    0x81, 0x02,        //   Input (Data, Variable, Absolute)
    0x09, 0x21,        //   Usage (Output Report Data)
    0x15, 0x00,        //   Logical Minimum (0)
    0x26, 0xFF, 0x00,  //   Logical Maximum (255)
    0x75, 0x08,        //   Report Size (8 bits)
    0x95, 0x40,        //   Report Count (64 bytes)
    0x91, 0x02,        //   Output (Data, Variable, Absolute)
    0xC0,              // End Collection
}

// Implementation of Start and Stop using purego

var deviceObj uintptr
var delegateObj uintptr
var delegateClass uintptr

func setupDelegateClass() {
    className := "GoHIDVirtualDeviceDelegate"
    if c := objc_getClass(className); c != 0 {
        delegateClass = c
        return
    }

    super := objc_getClass("NSObject")
    delegateClass = objc_allocateClassPair(super, className, 0)
    
    // Add methods
    // - (void)hidVirtualDevice:(HIDVirtualDevice *)device receivedSetReportRequestOfType:(HIDReportType)type id:(HIDReportID)reportID data:(NSData *)data
    // Selector: hidVirtualDevice:receivedSetReportRequestOfType:id:data:
    // Signature: v@:@Q@  (void, self, cmd, id, type(long), id(long), data)
    // Wait, types:
    // device: id
    // type: HIDReportType (likely NSInteger or int)
    // id: HIDReportID (likely NSInteger or int)
    // data: NSData *
    
    // Let's assume Type and ID are uint64 or int64 on arm64.
    
    // We use purego.NewCallback.
    // Definition: func(self uintptr, cmd uintptr, device uintptr, type uintptr, reportID uintptr, data uintptr)
    
    cb := purego.NewCallback(func(self, cmd, device, rType, reportID, data uintptr) {
        // Retrieve bytes from data (NSData)
        // [data bytes] -> pointer, [data length] -> int
        sel_bytes := sel_registerName("bytes")
        sel_length := sel_registerName("length")
        
        ptr := msgSend(data, sel_bytes)
        length := msgSend(data, sel_length)
        
        if ptr == 0 || length == 0 {
            return
        }
        
        // Copy to Go slice
        byteSlice := make([]byte, length)
        // Unsafe copy
        // We can cast ptr to *byte
        // Actually simplest is just to iterate or use unsafe.Slice if available (Go 1.17+)
        // Go 1.20+ has unsafe.Slice
        // Using a loop for safety if uncertain version, but unsafe.Slice is standard now.
        src := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)
        copy(byteSlice, src)
        
        hidLogger.Printf("RX Report (Type %d, ID %d): %x", rType, reportID, byteSlice)
        ctapHIDServer.HandleMessage(byteSlice)
    })
    
    class_addMethod(delegateClass, sel_registerName("hidVirtualDevice:receivedSetReportRequestOfType:id:data:"), cb, "v@:@qq@")
    
    objc_registerClassPair(delegateClass)
}

func PureGoStart(server *ctap_hid.CTAPHIDServer) {
    hidLogger.Println("Initializing PureGo HID Virtual Device...")
    
    // Diagnostic: Try a simple NSString call
    testStr := nsString("test")
    hidLogger.Printf("Diagnostic NSString created: %x\n", testStr)
    
    setupDelegateClass()
    
    // Create Properties Dictionary
    // Create Properties Dictionary
    // Keys - PascalCase based on IOKit conventions
    kDescriptor := nsString("ReportDescriptor")
    kVendorID := nsString("VendorID")
    
    hidLogger.Printf("Keys created (minimal): %x %x\n", kDescriptor, kVendorID)
    
    // Values
    vDescriptor := nsData(fidoReportDescriptor)
    vVendorID := nsNumber(10203) // 0x27DB
    
    // Create Dictionary
    cls_NSArray := objc_getClass("NSArray")
    sel_arrayWithObjectsCount := sel_registerName("arrayWithObjects:count:")
    
    keys := []uintptr{kDescriptor, kVendorID}
    vals := []uintptr{vDescriptor, vVendorID}
    
    nsKeys := msgSend(cls_NSArray, sel_arrayWithObjectsCount, uintptr(unsafe.Pointer(&keys[0])), uintptr(len(keys)))
    nsVals := msgSend(cls_NSArray, sel_arrayWithObjectsCount, uintptr(unsafe.Pointer(&vals[0])), uintptr(len(vals)))
    hidLogger.Printf("Arrays created: %x %x\n", nsKeys, nsVals)
    
    props := msgSend(cls_NSDictionary, sel_dictionaryWithObjectsForKeys, nsVals, nsKeys)
    hidLogger.Printf("Properties dictionary created: %x\n", props)
    
    // Init Device
    // device = [[HIDVirtualDevice alloc] initWithProperties:props]
    deviceObj = objc_alloc(cls_HIDVirtualDevice)
    hidLogger.Printf("Device allocated: %x\n", deviceObj)
    
    deviceObj = msgSend(deviceObj, sel_initWithProperties, props)
    hidLogger.Printf("Device initialized: %x\n", deviceObj)
    
    if deviceObj == 0 {
        hidLogger.Println("Failed to create HIDVirtualDevice via PureGo (alloc/init returned nil)")
        return
    }
    hidLogger.Printf("HIDVirtualDevice instance created: %x\n", deviceObj)

    // Init Delegate
    delegateObj = objc_alloc(delegateClass)
    delegateObj = msgSend(delegateObj, sel_init)
    hidLogger.Printf("Delegate instance created: %x\n", delegateObj)

    // Set Delegate
    msgSend(deviceObj, sel_setDelegate, delegateObj)
    hidLogger.Println("Delegate set")

    // Activate
    msgSend(deviceObj, sel_activate)
    
    hidLogger.Println("CoreHID device activated via PureGo")
}

func PureGoStop() {
    if deviceObj != 0 {
        msgSend(deviceObj, sel_cancel)
        deviceObj = 0
    }
}

func PureGoSend(data []byte) {
    if deviceObj == 0 {
        return
    }
    
    // Padding
    payload := make([]byte, 64)
    copy(payload, data)
    
    nsd := nsData(payload)
    
    // dispatchInputReport:timestamp:error:
    msgSend(deviceObj, sel_dispatchInputReport, nsd, 0, 0)
    
    // Pacing: 20ms
    // Logic moved here from Swift
    time.Sleep(20 * time.Millisecond)
}

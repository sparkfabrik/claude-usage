//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

void initTray(void);
void runApp(void);
void stopApp(void);
void setTrayTitle(const char *title, double r, double g, double b);
void setMenuItemTitle(int tag, const char *title);
void setMenuItemHidden(int tag, int hidden);
*/
import "C"
import "unsafe"

// Menu item tags — must match tray_darwin.m enum
const (
	tagStatus  = 1
	tag5h      = 2
	tagReset5h = 3
	tag7d      = 4
	tagReset7d = 5
	tagAuth    = 6
	tagError   = 7
	tagRefresh = 8
	tagQuit    = 9
)

func nativeInitTray() {
	C.initTray()
}

func nativeRunApp() {
	C.runApp()
}

func nativeStopApp() {
	C.stopApp()
}

func nativeSetTitle(title string, r, g, b float64) {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.setTrayTitle(cs, C.double(r), C.double(g), C.double(b))
}

func nativeSetMenuItemTitle(tag int, title string) {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.setMenuItemTitle(C.int(tag), cs)
}

func nativeSetMenuItemHidden(tag int, hidden bool) {
	h := 0
	if hidden {
		h = 1
	}
	C.setMenuItemHidden(C.int(tag), C.int(h))
}

//export goRefreshClicked
func goRefreshClicked() {
	select {
	case refreshCh <- struct{}{}:
	default:
	}
}

//export goQuitClicked
func goQuitClicked() {
	select {
	case quitCh <- struct{}{}:
	default:
	}
}

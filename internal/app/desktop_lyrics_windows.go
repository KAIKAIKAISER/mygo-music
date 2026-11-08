//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

var (
	modDwmapi                        = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute        = modDwmapi.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClientArea = modDwmapi.NewProc("DwmExtendFrameIntoClientArea")

	modUser32             = syscall.NewLazyDLL("user32.dll")
	procGetWindowLongW    = modUser32.NewProc("GetWindowLongW")
	procSetWindowLongW    = modUser32.NewProc("SetWindowLongW")
	procGetWindowLongPtrW = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = modUser32.NewProc("SetWindowLongPtrW")
	procSetWindowPos      = modUser32.NewProc("SetWindowPos")
)

type dwmMargins struct {
	cxLeftWidth    int32
	cxRightWidth   int32
	cyTopHeight    int32
	cyBottomHeight int32
}

const (
	dwmwaNcRenderingPolicy      = 2
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	dwmncrpDisabled  = 1
	dwmwcpDoNotRound = 1
	dwmColorNone     = 0xFFFFFFFE

	gwlExStyle = -20

	wsExToolWindow = 0x00000080

	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
)

func getWindowLong(hwnd uintptr, index int32) uintptr {
	if procGetWindowLongPtrW.Find() == nil {
		r, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(index))
		return r
	}
	r, _, _ := procGetWindowLongW.Call(hwnd, uintptr(index))
	return r
}

func setWindowLong(hwnd uintptr, index int32, value uintptr) {
	if procSetWindowLongPtrW.Find() == nil {
		procSetWindowLongPtrW.Call(hwnd, uintptr(index), value)
		return
	}
	procSetWindowLongW.Call(hwnd, uintptr(index), value)
}

func removeWindowBorder(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	// 1. DWM corner preference: DO NOT ROUND (removes Windows 11 rounded window frame/outline)
	corner := int32(dwmwcpDoNotRound)
	procDwmSetWindowAttribute.Call(hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&corner)), 4)

	// 2. DWM border color: NONE (0xFFFFFFFE)
	border := uint32(dwmColorNone)
	procDwmSetWindowAttribute.Call(hwnd, dwmwaBorderColor, uintptr(unsafe.Pointer(&border)), 4)

	// 3. DWM non-client rendering policy: DISABLED (1)
	policy := int32(dwmncrpDisabled)
	procDwmSetWindowAttribute.Call(hwnd, dwmwaNcRenderingPolicy, uintptr(unsafe.Pointer(&policy)), 4)

	// 4. Ensure no DWM frame margins are extended into the client area
	m := dwmMargins{}
	procDwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&m)))

	// 5. Ensure WS_EX_TOOLWINDOW is set so it doesn't show in Alt+Tab
	ex := getWindowLong(hwnd, gwlExStyle)
	if ex&wsExToolWindow == 0 {
		ex |= wsExToolWindow
		setWindowLong(hwnd, gwlExStyle, ex)
	}

	// 6. Notify Windows that frame styles/non-client areas have changed
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
}

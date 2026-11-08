//go:build !windows

package app

func removeWindowBorder(hwnd uintptr) {}

//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func lockFileExclusive(f *os.File) error {
	return lockFileEx(f, true)
}

func lockFileExclusiveBlocking(f *os.File) error {
	return lockFileEx(f, false)
}

func lockFileEx(f *os.File, failImmediately bool) error {
	const exclusive = 0x00000002
	const failImmediatelyFlag = 0x00000001
	flags := exclusive
	if failImmediately {
		flags |= failImmediatelyFlag
	}
	var ol syscall.Overlapped
	r1, _, e1 := procLockFileEx.Call(
		f.Fd(),
		uintptr(flags),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return e1
		}
		return syscall.EINVAL
	}
	return nil
}

func lockWouldBlock(err error) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	return ok && errno == syscall.Errno(33) // ERROR_LOCK_VIOLATION
}

func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r1, _, e1 := procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return e1
		}
		return syscall.EINVAL
	}
	return nil
}

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

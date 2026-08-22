//go:build unix

package main

import (
	"errors"
	"os"
	"syscall"
)

func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func lockFileExclusiveBlocking(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func lockWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}

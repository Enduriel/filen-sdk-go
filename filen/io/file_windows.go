//go:build windows

package io

import (
	"os"
	"syscall"
	"time"
)

func GetCreationTime(fileStat os.FileInfo) time.Time {
	return time.Unix(0, fileStat.Sys().(*syscall.Win32FileAttributeData).CreationTime.Nanoseconds())
}

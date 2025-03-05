//go:build !windows

package io

import (
	"os"
	"time"
)

func GetCreationTime(fileStat os.FileInfo) time.Time {
	return fileStat.ModTime()
}

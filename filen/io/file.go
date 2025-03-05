package io

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileInfo struct {
	Name         string
	MimeType     string
	Created      time.Time
	LastModified time.Time
	Data         io.Reader
}

func MakeInfoFromFile(file *os.File) (FileInfo, error) {
	fileStat, err := file.Stat()
	if err != nil {
		return FileInfo{}, fmt.Errorf("stat file: %w", err)
	}
	created := GetCreationTime(fileStat)
	return MakeFileInfo(file.Name(), created, fileStat.ModTime(), file, ""), nil
}

func MakeFileInfo(path string, created time.Time, lastModified time.Time, data io.Reader, mimeType string) FileInfo {
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(path))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	} else {
		mimeType, _, _ = strings.Cut(mimeType, ";")
	}
	return FileInfo{
		Name:         filepath.Base(path),
		MimeType:     mimeType,
		Created:      created.Round(time.Millisecond),
		LastModified: lastModified.Round(time.Millisecond),
		Data:         data,
	}
}

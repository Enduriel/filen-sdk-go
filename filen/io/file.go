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
	mimeType := mime.TypeByExtension(filepath.Ext(file.Name()))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	} else {
		mimeType, _, _ = strings.Cut(mimeType, ";")
	}
	return FileInfo{
		Name:         filepath.Base(file.Name()),
		MimeType:     mimeType,
		Created:      created.Round(time.Millisecond),
		LastModified: fileStat.ModTime().Round(time.Millisecond),
		Data:         file,
	}, nil
}

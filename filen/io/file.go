package io

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
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
	return FileInfo{
		Name:         filepath.Base(file.Name()),
		MimeType:     mime.TypeByExtension(filepath.Ext(file.Name())),
		Created:      created.Round(time.Millisecond),
		LastModified: fileStat.ModTime().Round(time.Millisecond),
		Data:         file,
	}, nil
}

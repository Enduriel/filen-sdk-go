package types

import (
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/io"
	"github.com/google/uuid"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IncompleteFile struct {
	UUID          string // the UUID of the cloud item
	Name          string
	MimeType      string
	EncryptionKey crypto.EncryptionKey // the key used to encrypt the file data
	Created       time.Time            // when the file was created
	LastModified  time.Time            // when the file was last modified
	ParentUUID    string               // the [Directory.UUID] of the file's parent directory
}

func NewIncompleteFile(authVersion int, name string, mimeType string, created time.Time, lastModified time.Time, parentUUID string) (*IncompleteFile, error) {
	key, err := crypto.MakeNewFileKey(authVersion)
	if err != nil {
		return nil, fmt.Errorf("make new file key: %w", err)
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	} else {
		mimeType, _, _ = strings.Cut(mimeType, ";")
	}

	return &IncompleteFile{
		UUID:          uuid.NewString(),
		Name:          filepath.Base(name),
		MimeType:      mimeType,
		EncryptionKey: *key,
		Created:       created.Round(time.Millisecond),
		LastModified:  lastModified.Round(time.Millisecond),
		ParentUUID:    parentUUID,
	}, nil
}

func NewIncompleteFileFromOSFile(authVersion int, osFile *os.File, parentUUID string) (*IncompleteFile, error) {
	fileStat, err := osFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	created := io.GetCreationTime(fileStat)
	return NewIncompleteFile(authVersion, osFile.Name(), "", created, fileStat.ModTime(), parentUUID)
}

// File represents a file on the cloud drive.
type File struct {
	IncompleteFile
	Size      int    // the file size in bytes
	Favorited bool   // whether the file is marked a favorite
	Region    string // the file's storage region
	Bucket    string // the file's storage bucket
	Chunks    int    // how many 1 MiB chunks the file is partitioned into
	Hash      string // the file's SHA512 hash
}

type DirColor string

const (
	DirColorDefault DirColor = ""
	DirColorBlue    DirColor = "blue"
	DirColorGreen   DirColor = "green"
	DirColorPurple  DirColor = "purple"
	DirColorRed     DirColor = "red"
	DirColorGray    DirColor = "gray"
)

type DirectoryMetaData struct {
	Name     string `json:"name"`
	Creation int    `json:"creation"`
}

// Directory represents a directory on the cloud drive.
type Directory struct {
	UUID       string    // the UUID of the cloud item
	Name       string    // the directory name
	ParentUUID string    // the [Directory.UUID] of the directory's parent directory (or zero value for the root directory)
	Color      DirColor  // the color assigned to the directory (zero value means default color)
	Created    time.Time // when the directory was created
	Favorited  bool      // whether the directory is marked a favorite
}

type RootDirectory struct {
	UUID string
}

type DirectoryInterface interface {
	GetUUID() string
	IsRoot() bool
}

type FileSystemObject interface {
	GetUUID() string
	GetParent() string
}

func (file *File) GetUUID() string {
	return file.IncompleteFile.UUID
}

func (file *File) GetParent() string {
	return file.ParentUUID
}

func (directory *Directory) GetUUID() string {
	return directory.UUID
}

func (directory *Directory) GetParent() string {
	return directory.ParentUUID
}

func (directory *Directory) IsRoot() bool {
	return false
}

func (rootDirectory *RootDirectory) GetUUID() string {
	return rootDirectory.UUID
}

func (rootDirectory *RootDirectory) GetParent() string {
	return ""
}

func (rootDirectory *RootDirectory) IsRoot() bool {
	return true
}

package filen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/util"
	"github.com/google/uuid"
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

func (api *Filen) MakeNewFileKey() (*crypto.EncryptionKey, error) {
	switch api.AuthVersion {
	case 1, 2:
		encryptionKeyStr := crypto.GenerateRandomString(32)
		encryptionKey, err := crypto.MakeEncryptionKeyFromBytes([32]byte([]byte(encryptionKeyStr)))
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 2: %w", err)
		}
		return encryptionKey, nil
	default:
		encryptionKey, err := crypto.NewEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 3: %w", err)
		}
		return encryptionKey, nil
	}
}

func (api *Filen) NewIncompleteFile(name string, mimeType string, created time.Time, lastModified time.Time, parentUUID string) (*IncompleteFile, error) {
	key, err := api.MakeNewFileKey()
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

func (api *Filen) NewIncompleteFileFromOSFile(osFile *os.File, parentUUID string) (*IncompleteFile, error) {
	fileStat, err := osFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	created := io.GetCreationTime(fileStat)
	return api.NewIncompleteFile(osFile.Name(), "", created, fileStat.ModTime(), parentUUID)
}

// File represents a file on the cloud drive.
type File struct {
	IncompleteFile
	Size      int    // the file size in bytes
	Favorited bool   // whether the file is marked a favorite
	Region    string // the file's storage region
	Bucket    string // the file's storage bucket
	Chunks    int    // how many 1 MiB chunks the file is partitioned into
}

// Directory represents a directory on the cloud drive.
type Directory struct {
	UUID       string    // the UUID of the cloud item
	Name       string    // the directory name
	ParentUUID string    // the [Directory.UUID] of the directory's parent directory (or zero value for the root directory)
	Color      string    // the color assigned to the directory (zero value means default color)
	Created    time.Time // when the directory was created
	Favorited  bool      // whether the directory is marked a favorite
}

// FindItemUUID finds a cloud item by its path and returns its UUID.
// Returns an empty string if none was found.
// Use this instead of FindItem to correctly handle paths pointing to the base directory.
func (api *Filen) FindItemUUID(path string, requireDirectory bool) (string, error) {
	if len(strings.Join(strings.Split(path, "/"), "")) == 0 { // empty path
		return api.BaseFolderUUID, nil
	} else {
		file, directory, err := api.FindItem(path, requireDirectory)
		if err != nil {
			return "", err
		}
		if file != nil {
			return file.UUID, nil
		}
		if directory != nil {
			return directory.UUID, nil
		}
		return "", nil
	}
}

// FindItem find a cloud item by its path and returns it (either the File or the Directory will be returned).
// Set requireDirectory to differentiate between files and directories with the same path (otherwise, the file will be found).
// Returns nil for both File and Directory if none was found.
func (api *Filen) FindItem(path string, requireDirectory bool) (*File, *Directory, error) {
	segments := strings.Split(path, "/")
	if len(strings.Join(segments, "")) == 0 {
		return nil, nil, fmt.Errorf("no segments in path %s", path)
	}

	currentUUID := api.BaseFolderUUID
SegmentsLoop:
	for segmentIdx, segment := range segments {
		if segment == "" {
			continue
		}

		files, directories, err := api.ReadDirectory(currentUUID)
		if err != nil {
			return nil, nil, fmt.Errorf("read directory: %w", err)
		}
		if !requireDirectory {
			for _, file := range files {
				if file.Name == segment {
					return file, nil, nil
				}
			}
		}
		for _, directory := range directories {
			if directory.Name == segment {
				if segmentIdx == len(segments)-1 {
					return nil, directory, nil
				} else {
					currentUUID = directory.UUID
					continue SegmentsLoop
				}
			}
		}
		return nil, nil, nil
	}
	return nil, nil, errors.New("unreachable")
}

// FindDirectoryOrCreate finds a cloud directory by its path and returns its UUID.
// If the directory cannot be found, it (and all non-existent parent directories) will be created.
func (api *Filen) FindDirectoryOrCreate(path string) (string, error) {
	segments := strings.Split(path, "/")
	if len(strings.Join(segments, "")) == 0 {
		return api.BaseFolderUUID, nil
	}

	currentUUID := api.BaseFolderUUID
SegmentsLoop:
	for _, segment := range segments {
		if segment == "" {
			continue
		}

		_, directories, err := api.ReadDirectory(currentUUID)
		if err != nil {
			return "", err
		}
		for _, directory := range directories {
			if directory.Name == segment {
				// directory found
				currentUUID = directory.UUID
				continue SegmentsLoop
			}
		}
		// create directory
		directory, err := api.CreateDirectory(currentUUID, segment)
		if err != nil {
			return "", err
		}
		currentUUID = directory.UUID
	}
	return currentUUID, nil
}

// ReadDirectory fetches the files and directories that are children of a directory (specified by UUID).
func (api *Filen) ReadDirectory(uuid string) ([]*File, []*Directory, error) {
	// fetch directory content
	directoryContent, err := api.client.PostV3DirContent(context.Background(), uuid)
	if err != nil {
		return nil, nil, fmt.Errorf("ReadDirectory fetching directory: %w", err)
	}

	// transform files
	files := make([]*File, 0)
	for _, file := range directoryContent.Uploads {
		metadataStr, err := api.DecryptMeta(file.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory decrypting metadata: %v", err)
		}
		var metadata FileMetadata
		err = json.Unmarshal([]byte(metadataStr), &metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory unmarshalling metadata: %v", err)
		}

		if len(metadata.Key) != 32 {

		}
		encryptionKey, err := crypto.MakeEncryptionKeyFromUnknownStr(metadata.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory creating encryption key: %v", err)
		}

		files = append(files, &File{
			IncompleteFile: IncompleteFile{
				UUID:          file.UUID,
				Name:          metadata.Name,
				MimeType:      metadata.MimeType,
				EncryptionKey: *encryptionKey,
				Created:       util.TimestampToTime(int64(metadata.Created)),
				LastModified:  util.TimestampToTime(int64(metadata.LastModified)),
				ParentUUID:    file.Parent,
			},
			Size:      metadata.Size,
			Favorited: file.Favorited == 1,
			Region:    file.Region,
			Bucket:    file.Bucket,
			Chunks:    file.Chunks,
		})
	}

	// transform directories
	directories := make([]*Directory, 0)
	for _, directory := range directoryContent.Folders {
		nameStr, err := api.DecryptMeta(directory.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory decrypting name: %v", err)
		}
		var name struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal([]byte(nameStr), &name)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory unmarshalling name: %v", err)
		}

		directories = append(directories, &Directory{
			UUID:       directory.UUID,
			Name:       name.Name,
			ParentUUID: directory.Parent,
			Color:      "<none>", //TODO tmp
			Created:    util.TimestampToTime(int64(directory.Timestamp)),
			Favorited:  directory.Favorited == 1,
		})
	}

	return files, directories, nil
}

// TrashFile moves a file to trash.
func (api *Filen) TrashFile(uuid string) error {
	return api.client.PostV3FileTrash(context.Background(), uuid)
}

// CreateDirectory creates a new directory.
func (api *Filen) CreateDirectory(parentUUID string, name string) (*Directory, error) {
	directoryUUID := uuid.New().String()

	// encrypt metadata
	metadata := struct {
		Name string `json:"name"`
	}{name}
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	metadataEncrypted := api.MasterKeys.EncryptMeta(string(metadataStr))

	// hash name
	nameHashed := api.HashFileName(name)

	// send
	response, err := api.client.PostV3DirCreate(context.Background(), directoryUUID, metadataEncrypted, nameHashed, parentUUID)
	if err != nil {
		return nil, err
	}
	return &Directory{
		UUID:       response.UUID,
		Name:       name,
		ParentUUID: parentUUID,
		Color:      "",
		Created:    time.Now(),
		Favorited:  false,
	}, nil
}

// TrashDirectory moves a directory to trash.
func (api *Filen) TrashDirectory(uuid string) error {
	return api.client.PostV3DirTrash(context.Background(), uuid)
}

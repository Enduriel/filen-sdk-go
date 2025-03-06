package filen

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

// DownloadToPath downloads a file from the cloud to the given downloadPath.
// The file is first downloaded to a temporary file in the same directory,
// then renamed to the final path. If an error occurs during download or rename,
// the temporary file is removed.
func (api *Filen) DownloadToPath(file *File, downloadPath string) error {
	downloadDir := path.Dir(downloadPath)
	// needs to be removed or renamed
	f, err := os.CreateTemp(downloadDir, fmt.Sprintf("%s-download-*.tmp", file.Name))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	fName := f.Name()
	err = api.downloadFile(file, f)
	errClose := f.Close()
	if err != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("download file: %w", err)
	}

	if errClose != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("close file: %w", errClose)
	}
	// should be okay because the temp file is in the same directory
	err = os.Rename(f.Name(), downloadPath)
	if err != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("rename file: %w", err)
	}
	return nil
}

func (api *Filen) UpdateMeta(file *File) error {
	metaData := FileMetadata{
		Name:         file.Name,
		Size:         file.Size,
		MimeType:     file.MimeType,
		Key:          file.EncryptionKey.ToStringWithAuthVersion(api.AuthVersion),
		LastModified: int(file.LastModified.UnixMilli()),
		Created:      int(file.Created.UnixMilli()),
	}
	metadataStr, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("marshal file metadata: %w", err)
	}
	metadataEncrypted := api.EncryptMeta(string(metadataStr))
	nameEncrypted := file.EncryptionKey.EncryptMeta(file.Name)
	nameHashed := api.HashFileName(file.Name)

	err = api.client.PostV3FileMetadata(file.UUID, nameEncrypted, nameHashed, metadataEncrypted)
	if err != nil {
		return fmt.Errorf("post v3 file metadata: %w", err)
	}
	return nil
}

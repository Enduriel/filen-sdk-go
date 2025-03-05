package filen

import (
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

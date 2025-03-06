package filen_sdk_go

import (
	"bytes"
	"fmt"
	sdk "github.com/FilenCloudDienste/filen-sdk-go/filen"
	filenio "github.com/FilenCloudDienste/filen-sdk-go/filen/io"
	"github.com/joho/godotenv"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

var filen *sdk.Filen

func setupEnv() error {

	err := godotenv.Load()
	if err != nil {
		// we don't panic in case the environment variables are set somewhere else
		println("Warning: Error loading .env file: ", err.Error())
	}

	email := os.Getenv("TEST_EMAIL")       // todo fill from env
	password := os.Getenv("TEST_PASSWORD") // todo fill from env
	filen, err = sdk.New(email, password)
	if err != nil {
		panic(err)
	}
	if err = writeTestFiles(); err != nil {
		return err
	}
	return nil
}

func TestMain(m *testing.M) {
	// prep client
	err := setupEnv()
	if err != nil {
		panic(err)
	}

	// run tests
	code := m.Run()
	os.Exit(code)
}

// TestReadDirectories  expects root directory to contain
//   - large_sample-1mb.txt
//   - abc.txt
//   - /def
//   - /uploads
func TestReadDirectories(t *testing.T) {
	files, dirs, err := filen.ReadDirectory(filen.BaseFolderUUID)
	if err != nil {
		t.Fatal(err)
	}

	requiredDirs := map[string]struct{}{"def": {}, "uploads": {}} // ("def" ), "uploads"}
	for _, dir := range dirs {
		if _, ok := requiredDirs[dir.Name]; ok {
			delete(requiredDirs, dir.Name)
		} else {
			t.Fatalf("Unexpected directory: %#v\n", dir)
		}
	}

	if len(requiredDirs) > 0 {
		t.Fatalf("Missing directories: %v\n", requiredDirs)
	}

	requiredFiles := map[string]struct{}{"large_sample-1mb.txt": {}, "abc.txt": {}}

	for _, file := range files {
		if _, ok := requiredFiles[file.Name]; ok {
			delete(requiredFiles, file.Name)
		} else {
			fmt.Printf("Unexpected file: %#v\n", file)
		}
	}
	if len(requiredFiles) > 0 {
		t.Fatalf("Missing files: %v\n", requiredFiles)
	}
}

func TestEmptyFileActions(t *testing.T) {
	osFile, err := os.Open("test_files/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	uploadFileInfo, err := filenio.MakeInfoFromFile(osFile)
	if err != nil {
		t.Fatal(err)
	}
	var file *sdk.File

	if !t.Run("Upload", func(t *testing.T) {
		file, err = filen.UploadFile(uploadFileInfo, filen.BaseFolderUUID)
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}

	t.Run("Find", func(t *testing.T) {
		foundFile, _, err := filen.FindItem("/empty.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		if foundFile.Size != 0 {
			t.Fatalf("File size is not zero: %v", foundFile.Size)
		}
	})

	t.Run("Download", func(t *testing.T) {
		err = filen.DownloadToPath(file, "downloaded/empty.txt")
		if err != nil {
			t.Fatal(err)
		}
		downloadedFile, err := os.Open("downloaded/empty.txt")
		if err != nil {
			t.Fatal(err)
		}
		fileData, err := io.ReadAll(downloadedFile)
		if err != nil {
			t.Fatal(err)
		}
		if len(fileData) != 0 {
			t.Fatalf("File size is not zero: %v", len(fileData))
		}
	})
	t.Run("Trash", func(t *testing.T) {
		err = filen.TrashFile(file.UUID)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestFileActions(t *testing.T) {
	osFile, err := os.Open("test_files/large_sample-3mb.txt")

	uploadsDirUUID := filen.BaseFolderUUID
	uploadFileInfo, err := filenio.MakeInfoFromFile(osFile)
	if err != nil {
		t.Fatal(err)
	}

	var (
		file *sdk.File
	)

	if !t.Run("Upload", func(t *testing.T) {
		file, err = filen.UploadFile(uploadFileInfo, uploadsDirUUID)
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}

	t.Run("ChangeMeta", func(t *testing.T) {
		file.Created = file.Created.Add(time.Second)
		file.LastModified = file.LastModified.Add(time.Second)

		err = filen.UpdateMeta(file)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Find", func(t *testing.T) {
		foundFile, _, err := filen.FindItem("/large_sample-3mb.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(file, foundFile) {
			t.Fatalf("Uploaded \n%#v\n and Downloaded \n%#v\n file info did not match", file, foundFile)
		}
	})

	t.Run("Download", func(t *testing.T) {
		err := filen.DownloadToPath(file, "downloaded/large_sample-3mb.txt")
		if err != nil {
			t.Fatal(err)
		}
		downloadedFile, err := os.Open("downloaded/large_sample-3mb.txt")
		if err != nil {
			t.Fatal(err)
		}
		eq, err := assertFilesEqual(osFile, downloadedFile)
		if err != nil {
			t.Fatal(err)
		}
		if !eq {
			t.Fatalf("Uploaded \n%#v\n and downloaded file contents did not match", file)
		}
	})

	t.Run("Trash", func(t *testing.T) {
		err = filen.TrashFile(file.UUID)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func writeTestData(writer io.Writer, length int) error {
	data := make([]byte, 0)
	for i := 0; i < length; i++ {
		data = append(data, []byte(fmt.Sprintf("%v\n", i))...)
	}
	_, err := writer.Write(data)
	return err
}

func writeTestFiles() error {
	smallFile, err := os.Create("test_files/large_sample-1mb.txt")
	if err != nil {
		return fmt.Errorf("failed to create large_sample-1mb: %w", err)
	}
	defer func() { _ = smallFile.Close() }()
	if err = writeTestData(smallFile, 100_000); err != nil {
		return err
	}
	bigFile, err := os.Create("test_files/large_sample-3mb.txt")
	if err != nil {
		return fmt.Errorf("failed to create large_sample-3mb: %w", err)
	}
	if err = writeTestData(bigFile, 350_000); err != nil {
		return err
	}
	return nil
}

func assertFilesEqual(f1 *os.File, f2 *os.File) (bool, error) {
	const chunkSize = 1024
	b1 := make([]byte, chunkSize)
	b2 := make([]byte, chunkSize)
	i := 0
	_, err1 := f1.Seek(0, io.SeekStart)
	_, err2 := f2.Seek(0, io.SeekStart)

	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("seek error: %v, %v", err1, err2)
	}
	for {
		i++
		_, err1 = f1.Read(b1)
		_, err2 = f2.Read(b2)

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true, nil
			} else if err1 == io.EOF || err2 == io.EOF {
				return false, nil
			} else {
				return false, fmt.Errorf("read error: %v, %v", err1, err2)
			}
		}

		if !bytes.Equal(b1, b2) {
			fmt.Printf("Chunk %d did not match\n", i)
			fmt.Printf("b1: %v\nb2: %v\n", b1, b2)
			return false, nil
		}
	}
}

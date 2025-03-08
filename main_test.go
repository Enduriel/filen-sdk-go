package filen_sdk_go

import (
	"bytes"
	"context"
	"fmt"
	sdk "github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
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
	files, dirs, err := filen.ReadDirectory(context.Background(), filen.BaseFolderUUID)
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

func TestDirectoryActions(t *testing.T) {
	newPath := "/abc/def/ghi"
	var directory *types.Directory
	t.Run("Create FindDirectoryOrCreate", func(t *testing.T) {
		dirOrRoot, err := filen.FindDirectoryOrCreate(context.Background(), newPath)
		if err != nil {
			t.Fatal(err)
		}
		if dir, ok := dirOrRoot.(*types.Directory); ok {
			directory = dir
		} else {
			t.Fatal("dirOrRoot is not a normal directory")
		}
	})

	t.Run("Find FindDirectoryOrCreate", func(t *testing.T) {
		dirOrRoot, err := filen.FindDirectoryOrCreate(context.Background(), newPath)
		if err != nil {
			t.Fatal(err)
		}
		if dir, ok := dirOrRoot.(*types.Directory); ok {
			dir.Created = time.Time{} // timestamps are set by server
			if !reflect.DeepEqual(dir, directory) {
				t.Fatalf("directories are not equal:\nCreated:%#v\nFound:%#v\n", directory, dir)
			}
		} else {
			t.Fatal("dirOrRoot is not a normal directory")
		}
	})
	t.Run("Trash", func(t *testing.T) {
		err := filen.TrashDirectory(context.Background(), directory.UUID)
		if err != nil {
			t.Fatal(err)
		}

		dir, err := filen.FindItem(context.Background(), newPath, true)
		if err != nil {
			t.Fatal("failed to gracefully handle missing directory: ", err)
		}
		if dir != nil {
			t.Fatal("Directory not trashed")
		}
	})
	t.Run("Cleanup", func(t *testing.T) {
		dir, err := filen.FindItem(context.Background(), "/abc", true)
		if err != nil {
			t.Fatal(err)
		}
		if dir, ok := dir.(*types.Directory); ok {
			err := filen.TrashDirectory(context.Background(), dir.UUID)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestEmptyFileActions(t *testing.T) {
	osFile, err := os.Open("test_files/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	incompleteFile, err := types.NewIncompleteFileFromOSFile(filen.AuthVersion, osFile, filen.BaseFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	var file *types.File

	if !t.Run("Upload", func(t *testing.T) {
		file, err = filen.UploadFile(context.Background(), incompleteFile, osFile)
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}

	t.Run("Find", func(t *testing.T) {
		foundObj, err := filen.FindItem(context.Background(), "/empty.txt", false)

		if err != nil {
			t.Fatal(err)
		}
		if foundObj == nil {
			t.Fatal("File not found")
		}
		foundFile, ok := foundObj.(*types.File)
		if !ok {
			t.Fatal("File not found")
		}
		if foundFile.Size != 0 {
			t.Fatalf("File size is not zero: %v", foundFile.Size)
		}
	})

	t.Run("Download", func(t *testing.T) {
		err = filen.DownloadToPath(context.Background(), file, "downloaded/empty.txt")
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
		err = filen.TrashFile(context.Background(), file.UUID)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestFileActions(t *testing.T) {
	fileName := "large_sample-20mb.txt"
	osFile, err := os.Open("test_files/" + fileName)

	uploadsDirUUID := filen.BaseFolderUUID
	incompleteFile, err := types.NewIncompleteFileFromOSFile(filen.AuthVersion, osFile, uploadsDirUUID)
	if err != nil {
		t.Fatal(err)
	}

	var (
		file *types.File
	)

	if !t.Run("Upload", func(t *testing.T) {
		file, err = filen.UploadFile(context.Background(), incompleteFile, osFile)
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}

	t.Run("ChangeMeta", func(t *testing.T) {
		file.Created = file.Created.Add(time.Second)
		file.LastModified = file.LastModified.Add(time.Second)

		err = filen.UpdateMeta(context.Background(), file)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Find", func(t *testing.T) {
		foundObj, err := filen.FindItem(context.Background(), "/"+fileName, false)
		if err != nil {
			t.Fatal(err)
		}
		foundFile, ok := foundObj.(*types.File)
		if !ok {
			t.Fatal("File not found")
		}
		if !reflect.DeepEqual(file, foundFile) {
			t.Fatalf("Uploaded \n%#v\n and Downloaded \n%#v\n file info did not match", file, foundFile)
		}
	})

	t.Run("Download", func(t *testing.T) {
		downloadPath := "downloaded/" + fileName
		err := filen.DownloadToPath(context.Background(), file, downloadPath)
		if err != nil {
			t.Fatal(err)
		}
		downloadedFile, err := os.Open(downloadPath)
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
		err = filen.TrashFile(context.Background(), file.UUID)
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
	normalFile, err := os.Create("test_files/large_sample-3mb.txt")
	if err != nil {
		return fmt.Errorf("failed to create large_sample-3mb: %w", err)
	}
	if err = writeTestData(normalFile, 350_000); err != nil {
		return err
	}
	bigFile, err := os.Create("test_files/large_sample-20mb.txt")
	if err != nil {
		return fmt.Errorf("failed to create large_sample-20mb: %w", err)
	}
	if err = writeTestData(bigFile, 2_700_000); err != nil {
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

package filen_sdk_go

import (
	"fmt"
	sdk "github.com/FilenCloudDienste/filen-sdk-go/filen"
	"github.com/joho/godotenv"
	"io"
	"os"
	"testing"
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

func TestFileActions(t *testing.T) {
	osFile, err := os.Open("test_files/large_sample-3mb.txt")

	uploadsDirUUID := filen.BaseFolderUUID
	fileName := "large_sample-3mb.txt"
	var (
		file      *sdk.File
		foundFile *sdk.File
	)

	if !t.Run("Upload", func(t *testing.T) {
		file, err = filen.UploadFile(fileName, uploadsDirUUID, osFile)
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}

	t.Run("Find", func(t *testing.T) {
		f, _, err := filen.FindItem("/large_sample-3mb.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		foundFile = f // this shouldn't be necessary and should be removed when below is fixed
		// TODO fix as this this failing for some reason
		//if !reflect.DeepEqual(file, foundFile) {
		//	t.Fatalf("Uploaded \n%#v\n and Downloaded \n%#v\n file info did not match", file, foundFile)
		//}
	})

	t.Run("Download", func(t *testing.T) {
		downloadFile, err := os.Create("downloaded/large_sample-3mb.txt")
		if err != nil {
			t.Fatal(err)
		}
		err = filen.DownloadFile(foundFile, downloadFile)
		if err != nil {
			t.Fatal(err)
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

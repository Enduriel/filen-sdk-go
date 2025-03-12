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
	"path/filepath"
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

	email := os.Getenv("TEST_EMAIL")
	password := os.Getenv("TEST_PASSWORD")
	if email == "" || password == "" {
		return fmt.Errorf("TEST_EMAIL and TEST_PASSWORD environment variables must be set")
	}
	filen, err = sdk.New(context.Background(), email, password)
	if err != nil {
		panic(err)
	}
	testPath := filepath.Join(".", "test_files")
	err = os.MkdirAll(testPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating test_files directory: %w", err)
	}
	if err = writeTestFiles(); err != nil {
		return err
	}
	downloadedPath := filepath.Join(".", "downloaded")
	err = os.MkdirAll(downloadedPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("creating downloaded directory: %w", err)
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

// TestUploadsToGoDir uploads test files and directories to the "go" directory
// this is so the TS sdk can validate these files on its side and check if they are compatible
// there should ideally be a TestDownloadsFromTSDir that validates files from the TS sdk
func TestUploadsToGoDir(t *testing.T) {
	goDir, err := filen.FindDirectoryOrCreate(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	testDir, err := filen.FindDirectory(context.Background(), "go/test")
	if err != nil {
		t.Fatal(err)
	}
	if testDir != nil {
		err = filen.TrashDirectory(context.Background(), testDir)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = filen.CreateDirectory(context.Background(), goDir, "test")
	if err != nil {
		t.Fatal(err)
	}
	testFile, err := types.NewIncompleteFile(filen.AuthVersion, "hello.txt", "", time.Now(), time.Now(), goDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = filen.UploadFile(context.Background(), testFile, bytes.NewReader([]byte("Hello World From Go!")))
	if err != nil {
		t.Fatal(err)
	}
	testEmptyFile, err := types.NewIncompleteFile(filen.AuthVersion, "empty.txt", "", time.Now(), time.Now(), goDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = filen.UploadFile(context.Background(), testEmptyFile, bytes.NewReader([]byte("")))
	if err != nil {
		t.Fatal(err)
	}
	testBigFile, err := types.NewIncompleteFile(filen.AuthVersion, "large_sample-20mb.txt", "", time.Now(), time.Now(), goDir)
	if err != nil {
		t.Fatal(err)
	}
	osFile, err := os.Open("test_files/large_sample-20mb.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = filen.UploadFile(context.Background(), testBigFile, osFile)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadDirectories(t *testing.T) {
	expectedDirs := map[string]*types.Directory{}
	expectedFiles := map[string]*types.File{}

	t.Run("setup", func(t *testing.T) {
		var err error
		def, err := filen.CreateDirectory(context.Background(), &filen.BaseFolder, "def")
		if err != nil {
			t.Fatal(err)
		}
		expectedDirs["def"] = def
		uploads, err := filen.CreateDirectory(context.Background(), &filen.BaseFolder, "uploads")
		if err != nil {
			t.Fatal(err)
		}
		expectedDirs["uploads"] = uploads
		incompleteFile, err := types.NewIncompleteFile(filen.AuthVersion, "large_sample-1mb.txt", "", time.Now(), time.Now(), &filen.BaseFolder)
		if err != nil {
			t.Fatal(err)
		}
		largeSample, err := filen.UploadFile(context.Background(), incompleteFile, bytes.NewReader([]byte("Sample!")))
		if err != nil {
			t.Fatal(err)
		}
		expectedFiles["large_sample-1mb.txt"] = largeSample
		incompleteFile, err = types.NewIncompleteFile(filen.AuthVersion, "abc.txt", "", time.Now(), time.Now(), &filen.BaseFolder)
		if err != nil {
			t.Fatal(err)
		}
		abc, err := filen.UploadFile(context.Background(), incompleteFile, bytes.NewReader([]byte("ABC!")))
		if err != nil {
			t.Fatal(err)
		}
		expectedFiles["abc.txt"] = abc
	})

	requiredDirs := map[string]*types.Directory{}
	requiredFiles := map[string]*types.File{}

	for k, v := range expectedDirs {
		requiredDirs[k] = v
	}
	for k, v := range expectedFiles {
		requiredFiles[k] = v
	}

	t.Run("Check", func(t *testing.T) {
		files, dirs, err := filen.ReadDirectory(context.Background(), filen.BaseFolder)
		if err != nil {
			t.Fatal(err)
		}

		for _, dir := range dirs {
			if requiredDir, ok := requiredDirs[dir.Name]; ok {
				if !reflect.DeepEqual(dir, requiredDir) {
					t.Fatalf("Directory %s does not match found:\n%#v\nExpected:\n%#v\n", dir.Name, dir, requiredDir)
				}
				delete(requiredDirs, dir.Name)
			}
		}

		if len(requiredDirs) > 0 {
			t.Fatalf("Missing directories: %v\n", requiredDirs)
		}

		for _, file := range files {
			if requiredFile, ok := requiredFiles[file.Name]; ok {
				if !reflect.DeepEqual(file, requiredFile) {
					t.Fatalf("File %s does not match found:\n%#v\nExpected:\n%#v\n", file.Name, file, requiredFile)
				}
				delete(requiredFiles, file.Name)
			}
		}
		if len(requiredFiles) > 0 {
			t.Fatalf("Missing files: %v\n", requiredFiles)
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		for _, dir := range expectedDirs {
			err := filen.TrashDirectory(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
		}

		for _, file := range expectedFiles {
			err := filen.TrashFile(context.Background(), *file)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestSerialization(t *testing.T) {
	//osFile, err := os.Create("cache/login.filen")
	//if err != nil {
	//	t.Fatal(err)
	//}
	buffer := bytes.NewBuffer(make([]byte, 0, 1024*1024))
	err := filen.SerializeTo(buffer)
	if err != nil {
		t.Fatal(err)
	}
	//_, err = osFile.Seek(0, io.SeekStart)
	//if err != nil {
	//	t.Fatal(err)
	//}
	deserialized, err := sdk.DeserializeFrom(buffer)
	if err != nil {
		t.Fatal(err)
	}
	deserializedClient := deserialized.Client
	deserialized.Client = filen.Client
	if !reflect.DeepEqual(filen, deserialized) {
		t.Fatalf("Filen objects are not equal:\nOriginal:%#v\nDeserialized:%#v\n", filen, deserialized)
	}
	if filen.Client.APIKey != deserializedClient.APIKey {
		t.Fatalf("API keys are not equal:\nOriginal:%#v\nDeserialized:%#v\n", filen.Client.APIKey, deserializedClient.APIKey)
	}
}

func TestDirectoryActions(t *testing.T) {
	newPath := "/abc/def/ghi"
	var directory *types.Directory
	t.Run("GetBaseFolder", func(t *testing.T) {
		dirOrRoot, err := filen.FindDirectory(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if dir, ok := dirOrRoot.(*types.RootDirectory); ok {
			if dir.GetUUID() != filen.BaseFolder.GetUUID() {
				t.Fatalf("root directory did not match")
			}
		} else {
			t.Fatal("dirOrRoot is not a root directory")
		}
	})
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
			if !reflect.DeepEqual(dir, directory) {
				t.Fatalf("directories are not equal:\nCreated:%#v\nFound:%#v\n", directory, dir)
			}
		} else {
			t.Fatal("dirOrRoot is not a normal directory")
		}
	})
	t.Run("Trash", func(t *testing.T) {
		err := filen.TrashDirectory(context.Background(), directory)
		if err != nil {
			t.Fatal(err)
		}

		dir, err := filen.FindDirectory(context.Background(), newPath)
		if err != nil {
			t.Fatal("failed to gracefully handle missing directory: ", err)
		}
		if dir != nil {
			t.Fatal("Directory not trashed")
		}
	})
	t.Run("Cleanup", func(t *testing.T) {
		dir, err := filen.FindDirectory(context.Background(), "/abc")
		if err != nil {
			t.Fatal(err)
		}
		err = filen.TrashDirectory(context.Background(), dir)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestEmptyFileActions(t *testing.T) {
	osFile, err := os.Open("test_files/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	incompleteFile, err := types.NewIncompleteFileFromOSFile(filen.AuthVersion, osFile, filen.BaseFolder)
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
		foundObj, err := filen.FindItem(context.Background(), "empty.txt", false)

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
		err = filen.TrashFile(context.Background(), *file)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestNewWithAPIKey(t *testing.T) {
	filen2, err := sdk.NewWithAPIKey(context.Background(), os.Getenv("TEST_EMAIL"), os.Getenv("TEST_PASSWORD"), filen.GetAPIKey())
	if err != nil {
		t.Fatal(err)
	}
	newClient := filen2.Client
	filen2.Client = filen.Client
	if !reflect.DeepEqual(filen, filen2) {
		t.Fatalf("Filen objects are not equal:\nOriginal:%#v\nDeserialized:%#v\n", filen, filen2)
	}
	if filen.Client.APIKey != newClient.APIKey {
		t.Fatalf("API keys are not equal:\nOriginal:%#v\nDeserialized:%#v\n", filen.Client.APIKey, filen2.Client.APIKey)
	}
}

func TestFileActions(t *testing.T) {
	fileName := "large_sample-20mb.txt"
	osFile, err := os.Open("test_files/" + fileName)

	incompleteFile, err := types.NewIncompleteFileFromOSFile(filen.AuthVersion, osFile, filen.BaseFolder)
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
		foundObj, err := filen.FindItem(context.Background(), fileName, false)
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
		err = filen.TrashFile(context.Background(), *file)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestPartialRead(t *testing.T) {
	fileName := "partial_read.txt"

	incompleteFile, err := types.NewIncompleteFile(filen.AuthVersion, fileName, "", time.Now(), time.Now(), &filen.BaseFolder)
	if err != nil {
		t.Fatal(err)
	}

	myString := "Sample!"
	contents := make([]byte, sdk.ChunkSize+len(myString))
	copy(contents[:], myString)
	copy(contents[sdk.ChunkSize:], myString)

	file, err := filen.UploadFile(context.Background(), incompleteFile, bytes.NewReader(contents))
	if err != nil {
		t.Fatal(err)
	}

	reader := filen.GetDownloadReaderWithOffset(context.Background(), file, 0, len(myString))

	readBytes := make([]byte, sdk.ChunkSize*2)
	n, err := reader.Read(readBytes)
	if err != io.EOF && err != nil {
		t.Fatal(err)
	}

	if string(readBytes[:n]) != myString {
		t.Fatalf("Expected %s, got %s", myString, string(readBytes[:n]))
	}

	reader = filen.GetDownloadReaderWithOffset(context.Background(), file, sdk.ChunkSize, sdk.ChunkSize*2)
	readBytes = make([]byte, sdk.ChunkSize*2)
	n, err = reader.Read(readBytes)
	if err != io.EOF && err != nil {
		t.Fatal(err)
	}

	if string(readBytes[:n]) != myString {
		t.Fatalf("Expected %s, got %s", myString, string(readBytes[:n]))
	}

	err = filen.TrashFile(context.Background(), *file)
	if err != nil {
		t.Fatal(err)
	}
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
	_, err := os.Create("test_files/empty.txt")
	if err != nil {
		return fmt.Errorf("failed to create empty.txt: %w", err)
	}
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

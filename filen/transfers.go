package filen

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	filenio "github.com/FilenCloudDienste/filen-sdk-go/filen/io"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"io"
	"strconv"
	"strings"
)

const (
	maxNetworkWorkers    = 16
	maxDownloadedBuffer  = 16
	maxCryptoWorkers     = 16
	maxCryptoedBuffer    = 16
	maxReadBuffer        = 16
	maxConcurrentWriters = 16
	ChunkSize            = 1048576
)

type FileUpload struct {
	UUID string
	// needed for chunk upload
	uploadKey     string
	encryptionKey crypto.EncryptionKey
	ctx           context.Context
	filen         *Filen
	// needed for file metadata
	ParentUUID string
	fileInfo   filenio.FileInfo
}

type Chunk struct {
	Index int
	Data  []byte
}

func NewFileUpload(filen *Filen, fileInfo filenio.FileInfo, parentUUID string, ctx context.Context) (*FileUpload, error) {

	// TODO check if this is the correct approach
	var (
		encryptionKey *crypto.EncryptionKey
		err           error
	)
	if filen.AuthVersion == 2 || filen.AuthVersion == 1 {
		encryptionKeyStr := crypto.GenerateRandomString(32)
		encryptionKey, err = crypto.MakeEncryptionKeyFromBytes([32]byte([]byte(encryptionKeyStr)))
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 2: %w", err)
		}
	} else if filen.AuthVersion == 3 {
		encryptionKey, err = crypto.NewEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 3: %w", err)
		}
	} else {
		panic("unknown auth version")
	}

	return &FileUpload{
		UUID:          uuid.New().String(),
		fileInfo:      fileInfo,
		uploadKey:     crypto.GenerateRandomString(32),
		encryptionKey: *encryptionKey,
		ctx:           ctx,
		filen:         filen,
		ParentUUID:    parentUUID,
	}, nil
}

func (fu *FileUpload) readChunks(inReader io.Reader, outChunks chan<- Chunk) (int, error) {
	defer close(outChunks)
	chunkID := 0
	buffer := make([]byte, ChunkSize)
	size := 0

	for {
		select {
		case <-fu.ctx.Done():
			return 0, fu.ctx.Err()
		default:
			n, err := inReader.Read(buffer)
			if err == io.EOF {
				return size, nil
			}
			if err != nil {
				return 0, fmt.Errorf("read chunk %d: %w", chunkID, err)
			}

			chunk := make([]byte, n, n+fu.encryptionKey.Cipher.Overhead())
			size += n
			copy(chunk, buffer[:n])
			select {
			case <-fu.ctx.Done():
				return 0, fu.ctx.Err()
			case outChunks <- Chunk{
				Index: chunkID,
				Data:  chunk,
			}:
				chunkID++
			}

		}
	}
}

func (fu *FileUpload) encryptChunks(in <-chan Chunk, out chan<- Chunk) error {
	defer close(out)
	g, ctx := errgroup.WithContext(fu.ctx)
	sem := make(chan struct{}, maxCryptoWorkers)

	for chunk := range in {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
			g.Go(func() error {
				defer func() { <-sem }()
				chunk.Data = fu.encryptionKey.EncryptData(chunk.Data)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- chunk:
					return nil
				}
			})
		}
	}
	return g.Wait()
}

func (fu *FileUpload) uploadChunks(in <-chan Chunk) (string, string, error) {
	g, ctx := errgroup.WithContext(fu.ctx)

	firstChunk := <-in
	if firstChunk.Data == nil {
		// zero sized file
		return "", "", nil
	}
	var (
		region string
		bucket string
	)

	// need to handle the first chunk separately
	g.Go(func() error {
		r, b, err := fu.filen.client.PostV3Upload(fu.ctx, fu.UUID, firstChunk.Index, fu.ParentUUID, fu.uploadKey, firstChunk.Data)
		if err != nil {
			return fmt.Errorf("upload first chunk: %w", err)
		}
		region = r
		bucket = b
		return nil
	})
	sem := make(chan struct{}, maxNetworkWorkers)

	for chunk := range in {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case sem <- struct{}{}:
			g.Go(func() error {
				defer func() { <-sem }()
				_, _, err := fu.filen.client.PostV3Upload(fu.ctx, fu.UUID, chunk.Index, fu.ParentUUID, fu.uploadKey, chunk.Data)
				if err != nil {
					return fmt.Errorf("upload chunk %d: %w", chunk.Index, err)
				}
				return nil
			})
		}
	}
	return region, bucket, g.Wait()
}

type FileMetadata struct {
	Name         string `json:"name"`
	Size         int    `json:"size"`
	MimeType     string `json:"mime"`
	Key          string `json:"key"`
	LastModified int    `json:"lastModified"`
	Created      int    `json:"created"`
}

func (fu *FileUpload) completeUpload(bucket string, region string, size int) (*File, error) {
	metadata := FileMetadata{
		Name:         fu.fileInfo.Name,
		Size:         size,
		MimeType:     fu.fileInfo.MimeType,
		Key:          fu.encryptionKey.ToStringWithAuthVersion(fu.filen.AuthVersion),
		LastModified: int(fu.fileInfo.LastModified.UnixMilli()),
		Created:      int(fu.fileInfo.Created.UnixMilli()),
	}
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}

	nameEncrypted := fu.encryptionKey.EncryptMeta(fu.fileInfo.Name)
	nameHashed := fu.filen.HashFileName(fu.fileInfo.Name)

	numChunks := (size / ChunkSize) + 1
	response, err := fu.filen.client.PostV3UploadDone(context.Background(), client.V3UploadDoneRequest{
		UUID:       fu.UUID,
		Name:       nameEncrypted,
		NameHashed: nameHashed,
		Size:       strconv.Itoa(size),
		Chunks:     numChunks,
		Metadata:   fu.filen.EncryptMeta(string(metadataStr)),
		MimeType:   fu.filen.EncryptMeta(fu.fileInfo.MimeType),
		Rm:         crypto.GenerateRandomString(32),
		Version:    fu.filen.AuthVersion,
		UploadKey:  fu.uploadKey,
	})

	if err != nil {
		return nil, fmt.Errorf("upload done: %w", err)
	}

	return &File{
		UUID:          fu.UUID,
		Name:          fu.fileInfo.Name,
		Size:          size,
		MimeType:      fu.fileInfo.MimeType,
		EncryptionKey: fu.encryptionKey,
		Created:       fu.fileInfo.Created,
		LastModified:  fu.fileInfo.LastModified,
		ParentUUID:    fu.ParentUUID,
		Favorited:     false,
		Region:        region,
		Bucket:        bucket,
		Chunks:        response.Chunks,
	}, nil
}

func (api *Filen) uploadEmptyFile(fu *FileUpload) (*File, error) {
	metadata := FileMetadata{
		Name:         fu.fileInfo.Name,
		Size:         0,
		MimeType:     fu.fileInfo.MimeType,
		Key:          fu.encryptionKey.ToStringWithAuthVersion(api.AuthVersion),
		LastModified: int(fu.fileInfo.LastModified.UnixMilli()),
		Created:      int(fu.fileInfo.Created.UnixMilli()),
	}

	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}

	_, err = api.client.PostV3UploadEmpty(context.Background(), client.V3UploadEmptyRequest{
		UUID:       fu.UUID,
		Name:       api.EncryptMeta(fu.fileInfo.Name),
		NameHashed: api.HashFileName(fu.fileInfo.Name),
		Size:       "0",
		Parent:     fu.ParentUUID,
		MimeType:   api.EncryptMeta(fu.fileInfo.MimeType),
		Metadata:   api.EncryptMeta(string(metadataStr)),
		Version:    api.AuthVersion,
	})

	if err != nil {
		return nil, fmt.Errorf("upload empty file: %w", err)
	}

	return &File{
		UUID:          fu.UUID,
		Name:          fu.fileInfo.Name,
		Size:          0,
		MimeType:      fu.fileInfo.MimeType,
		EncryptionKey: fu.encryptionKey,
		Created:       fu.fileInfo.Created,
		LastModified:  fu.fileInfo.LastModified,
		ParentUUID:    fu.ParentUUID,
		Favorited:     false,
		Region:        "",
		Bucket:        "",
		Chunks:        0,
	}, nil
}

func (api *Filen) UploadFile(fileInfo filenio.FileInfo, parentUUID string) (*File, error) {
	eg, ctx := errgroup.WithContext(context.Background())
	fileUpload, err := NewFileUpload(api, fileInfo, parentUUID, ctx)
	if err != nil {
		return nil, fmt.Errorf("new file upload: %w", err)
	}
	readChunks := make(chan Chunk, maxReadBuffer)
	encryptedChunks := make(chan Chunk, maxCryptoedBuffer)
	var (
		size   int
		region string
		bucket string
	)
	eg.Go(func() error {
		s, err := fileUpload.readChunks(fileInfo.Data, readChunks)
		if err != nil {
			return fmt.Errorf("read chunks: %w", err)
		}
		size = s
		return nil
	})
	eg.Go(func() error {
		if err := fileUpload.encryptChunks(readChunks, encryptedChunks); err != nil {
			return fmt.Errorf("encrypt chunks: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		r, b, err := fileUpload.uploadChunks(encryptedChunks)
		if err != nil {
			return fmt.Errorf("upload chunks: %w", err)
		}
		region = r
		bucket = b
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	if size == 0 {
		return api.uploadEmptyFile(fileUpload)
	}
	file, err := fileUpload.completeUpload(bucket, region, size)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return file, nil
	} else {
		return nil, err
	}
}

func (api *Filen) HashFileName(name string) string {
	name = strings.ToLower(name)
	switch api.AuthVersion {
	case 1, 2:
		outerHasher := sha1.New()
		innerHasher := sha256.New()
		innerHasher.Write([]byte(name))
		outerHasher.Write(innerHasher.Sum(nil))
		return hex.EncodeToString(outerHasher.Sum(nil))
	default:
		hasher := sha256.New()
		hasher.Write(api.DEK.Bytes[:])
		hasher.Write([]byte(name))
		return hex.EncodeToString(hasher.Sum(nil))
	}
}

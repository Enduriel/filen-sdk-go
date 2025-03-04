package filen

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"io"
	"strconv"
	"time"
)

const (
	maxNetworkWorkers    = 16
	maxDownloadedBuffer  = 16
	maxCryptoWorkers     = 16
	maxCryptoedBuffer    = 16
	maxReadBuffer        = 16
	maxConcurrentWriters = 16
	chunkSize            = 1048576
)

type fileDownload struct {
	file  *File
	ctx   context.Context
	filen *Filen
}

func newFileDownload(filen *Filen, file *File, ctx context.Context) *fileDownload {
	return &fileDownload{
		file:  file,
		ctx:   ctx,
		filen: filen,
	}
}

func (fd *fileDownload) downloadChunks(outChunks chan<- Chunk) error {
	defer close(outChunks)
	g, ctx := errgroup.WithContext(fd.ctx)
	sem := make(chan struct{}, maxNetworkWorkers)

	for i := 0; i < fd.file.Chunks; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
			g.Go(func() error {
				defer func() { <-sem }()
				fmt.Printf("Downloading chunk %d\n", i)
				encryptedBytes, err := fd.filen.client.DownloadFileChunk(fd.file.UUID, fd.file.Region, fd.file.Bucket, i)
				if err != nil {
					return fmt.Errorf("download i %d: %w", i, err)
				}
				outChunks <- Chunk{
					Data:  encryptedBytes,
					Index: i,
				}
				return nil
			})
		}
	}
	return g.Wait()
}

func (fd *fileDownload) decryptChunks(in <-chan Chunk, out chan<- Chunk) error {
	defer close(out)
	g, ctx := errgroup.WithContext(fd.ctx)
	sem := make(chan struct{}, maxCryptoWorkers)

	for chunk := range in {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
			g.Go(func() error {
				defer func() { <-sem }()
				fmt.Printf("Decrypting chunk %d\n", chunk.Index)

				decryptedBytes, err := fd.file.EncryptionKey.DecryptData(chunk.Data)
				if err != nil {
					return fmt.Errorf("decrypt chunk %d: %w", chunk.Index, err)
				}
				chunk.Data = decryptedBytes
				out <- chunk
				return nil
			})
		}
	}

	return g.Wait()
}

func (fd *fileDownload) writeChunks(in <-chan Chunk, ws io.WriteSeeker) error {
	for {
		select {
		case <-fd.ctx.Done():
			return fd.ctx.Err()
		case chunk, ok := <-in:
			if !ok {
				return nil
			}
			fmt.Printf("Writing chunk %d\n", chunk.Index)
			_, err := ws.Seek(int64(chunk.Index*chunkSize), io.SeekStart)
			if err != nil {
				return err
			}
			_, err = ws.Write(chunk.Data)
			if err != nil {
				return err
			}
		}
	}
}

func (api *Filen) DownloadFile(file *File, ws io.WriteSeeker) error {
	g, ctx := errgroup.WithContext(context.Background())
	fd := newFileDownload(api, file, ctx)
	downloadedChunks := make(chan Chunk, maxDownloadedBuffer)
	decryptedChunks := make(chan Chunk, maxCryptoedBuffer)

	g.Go(func() error {
		if err := fd.downloadChunks(downloadedChunks); err != nil {
			return fmt.Errorf("download chunks: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := fd.decryptChunks(downloadedChunks, decryptedChunks); err != nil {
			return fmt.Errorf("decrypt chunks: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := fd.writeChunks(decryptedChunks, ws); err != nil {
			return fmt.Errorf("write chunks: %w", err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

type FileUpload struct {
	UUID string
	// needed for chunk upload
	uploadKey     string
	encryptionKey crypto.EncryptionKey
	ctx           context.Context
	filen         *Filen
	// needed for file metadata
	ParentUUID string
}

type Chunk struct {
	Index int
	Data  []byte
}

func NewFileUpload(filen *Filen, parentUUID string, ctx context.Context) (*FileUpload, error) {

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
		ParentUUID:    parentUUID,
		uploadKey:     crypto.GenerateRandomString(32),
		encryptionKey: *encryptionKey,
		ctx:           ctx,
		filen:         filen,
	}, nil
}

func (fu *FileUpload) readChunks(inReader io.Reader, outChunks chan<- Chunk) (int, error) {
	defer close(outChunks)
	chunkID := 0
	buffer := make([]byte, chunkSize)
	size := 0

	for {
		select {
		case <-fu.ctx.Done():
			return 0, fu.ctx.Err()
		default:
			n, err := inReader.Read(buffer)
			if err == io.EOF {
				if size == 0 {
					return 0, errors.New("empty uploads are not supported")
				}
				return size, nil
			}
			if err != nil {
				return 0, fmt.Errorf("read chunk %d: %w", chunkID, err)
			}

			chunk := make([]byte, n)
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
	var (
		region string
		bucket string
	)

	// need to handle the first chunk separately
	g.Go(func() error {
		r, b, err := fu.filen.client.PostV3Upload(fu.UUID, firstChunk.Index, fu.ParentUUID, fu.uploadKey, firstChunk.Data)
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
				_, _, err := fu.filen.client.PostV3Upload(fu.UUID, chunk.Index, fu.ParentUUID, fu.uploadKey, chunk.Data)
				if err != nil {
					return fmt.Errorf("upload chunk %d: %w", chunk.Index, err)
				}
				return nil
			})
		}
	}
	return region, bucket, g.Wait()
}

func (fu *FileUpload) completeUpload(name string, bucket string, region string, size int) (*File, error) {
	metadata := struct {
		Name         string `json:"name"`
		Size         int    `json:"size"`
		MimeType     string `json:"mime"`
		Key          string `json:"key"`
		LastModified int    `json:"lastModified"`
		Created      int    `json:"created"`
		// TODO add hash, which is the hash of unencrypted bytes
	}{name, size, "text/plain", fu.encryptionKey.ToStringWithAuthVersion(fu.filen.AuthVersion), int(time.Now().Unix()), int(time.Now().Unix())}
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}

	metadataEncrypted := fu.filen.EncryptMeta(string(metadataStr))
	nameEncrypted := fu.filen.EncryptMeta(name)
	// TODO consider seeding this hash with the DEK
	nameHashed := hex.EncodeToString(crypto.RunSHA521([]byte(name)))

	numChunks := (size / chunkSize) + 1
	response, err := fu.filen.client.PostV3UploadDone(client.V3UploadDoneRequest{
		UUID:       fu.UUID,
		Name:       nameEncrypted,
		NameHashed: nameHashed,
		Size:       strconv.Itoa(size),
		Chunks:     numChunks,
		Metadata:   metadataEncrypted,
		MimeType:   "text/plain", // TODO figure out mime types
		Rm:         crypto.GenerateRandomString(32),
		Version:    2,
		UploadKey:  fu.uploadKey,
	})

	if err != nil {
		return nil, fmt.Errorf("upload done: %w", err)
	}

	return &File{
		UUID:          fu.UUID,
		Name:          name,
		Size:          int64(size),
		MimeType:      "application/octet-stream", //TODO correct mime type
		EncryptionKey: fu.encryptionKey,
		Created:       time.Now(), //TODO really?
		LastModified:  time.Now(),
		ParentUUID:    fu.ParentUUID,
		Favorited:     false,
		Region:        region,
		Bucket:        bucket,
		Chunks:        response.Chunks,
	}, nil

}

func (api *Filen) UploadFile(fileName string, parentUUID string, data io.Reader) (*File, error) {
	eg, ctx := errgroup.WithContext(context.Background())
	fileUpload, err := NewFileUpload(api, parentUUID, ctx)
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
		s, err := fileUpload.readChunks(data, readChunks)
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
	file, err := fileUpload.completeUpload(fileName, bucket, region, size)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return file, nil
	} else {
		return nil, err
	}
}

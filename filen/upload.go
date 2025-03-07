package filen

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"io"
	"strconv"
	"sync"
)

type FileUpload struct {
	IncompleteFile
	uploadKey string
	ctx       context.Context
	cancel    context.CancelCauseFunc
}

type FileMetadata struct {
	Name         string `json:"name"`
	Size         int    `json:"size"`
	MimeType     string `json:"mime"`
	Key          string `json:"key"`
	LastModified int    `json:"lastModified"`
	Created      int    `json:"created"`
}

func (api *Filen) newFileUpload(ctx context.Context, cancel context.CancelCauseFunc, file *IncompleteFile) *FileUpload {
	return &FileUpload{
		IncompleteFile: *file,
		uploadKey:      crypto.GenerateRandomString(32),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// TODO, do not forget to overallocate for data (encryption overhead)
func (api *Filen) uploadChunk(fu *FileUpload, chunkIndex int, data []byte) (*client.V3UploadResponse, error) {
	data = fu.EncryptionKey.EncryptData(data)
	response, err := api.client.PostV3Upload(fu.ctx, fu.UUID, chunkIndex, fu.ParentUUID, fu.uploadKey, data)
	if err != nil {
		return nil, fmt.Errorf("upload chunk %d: %w", chunkIndex, err)
	}
	return response, nil
}

func (api *Filen) makeEmptyRequestFromUploaderNoMeta(fu *FileUpload) *client.V3UploadEmptyRequest {
	return &client.V3UploadEmptyRequest{
		UUID:       fu.UUID,
		Name:       api.EncryptMeta(fu.Name),
		NameHashed: api.HashFileName(fu.Name),
		Size:       "0",
		Parent:     fu.ParentUUID,
		MimeType:   api.EncryptMeta(fu.MimeType),
		//Metadata: must be filled by caller
		Version: api.AuthVersion,
	}
}

func (api *Filen) makeEmptyRequestFromUploader(fu *FileUpload) (*client.V3UploadEmptyRequest, error) {
	metadata := FileMetadata{
		Name:         fu.Name,
		Size:         0,
		MimeType:     fu.MimeType,
		Key:          fu.EncryptionKey.ToStringWithAuthVersion(api.AuthVersion),
		LastModified: int(fu.LastModified.UnixMilli()),
		Created:      int(fu.Created.UnixMilli()),
	}

	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}
	emptyRequest := api.makeEmptyRequestFromUploaderNoMeta(fu)
	emptyRequest.Metadata = api.EncryptMeta(string(metadataStr))

	return emptyRequest, nil
}

func (api *Filen) makeRequestFromUploader(fu *FileUpload, size int) (*client.V3UploadDoneRequest, error) {
	metadata := FileMetadata{
		Name:         fu.Name,
		Size:         size,
		MimeType:     fu.MimeType,
		Key:          fu.EncryptionKey.ToStringWithAuthVersion(api.AuthVersion),
		LastModified: int(fu.LastModified.UnixMilli()),
		Created:      int(fu.Created.UnixMilli()),
	}

	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}
	emptyRequest := api.makeEmptyRequestFromUploaderNoMeta(fu)
	emptyRequest.Metadata = api.EncryptMeta(string(metadataStr))
	emptyRequest.Size = strconv.Itoa(size)

	return &client.V3UploadDoneRequest{
		V3UploadEmptyRequest: *emptyRequest,
		Chunks:               (size / ChunkSize) + 1,
		UploadKey:            fu.uploadKey,
		Rm:                   crypto.GenerateRandomString(32),
	}, nil
}

func (api *Filen) completeUpload(fu *FileUpload, bucket string, region string, size int) (*File, error) {
	uploadRequest, err := api.makeRequestFromUploader(fu, size)
	if err != nil {
		return nil, fmt.Errorf("make request from uploader: %w", err)
	}
	_, err = api.client.PostV3UploadDone(fu.ctx, *uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}

	return &File{
		IncompleteFile: fu.IncompleteFile,
		Size:           size,
		Region:         region,
		Bucket:         bucket,
		Chunks:         (size / ChunkSize) + 1,
	}, nil
}

func (api *Filen) completeUploadEmpty(fu *FileUpload) (*File, error) {
	uploadRequest, err := api.makeEmptyRequestFromUploader(fu)
	if err != nil {
		return nil, fmt.Errorf("make request from uploader: %w", err)
	}
	_, err = api.client.PostV3UploadEmpty(fu.ctx, *uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}

	return &File{
		IncompleteFile: fu.IncompleteFile,
		Size:           0,
		Region:         "",
		Bucket:         "",
	}, nil

}

func (api *Filen) UploadFile(ctx context.Context, file *IncompleteFile, r io.Reader) (*File, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil) // Ensure context is canceled when we exit

	fileUpload := api.newFileUpload(ctx, cancel, file)
	uploadSem := make(chan struct{}, MaxUploaders)
	wg := sync.WaitGroup{}
	bucketAndRegion := make(chan client.V3UploadResponse, 1)
	size := 0

	for i := 0; ; i++ {
		data := make([]byte, ChunkSize, ChunkSize+file.EncryptionKey.Cipher.Overhead())
		read, err := r.Read(data)
		size += read

		if err != nil && err != io.EOF {
			fileUpload.cancel(fmt.Errorf("read chunk %d: %w", i, err))
			return nil, err
		}

		if read > 0 {
			if read < ChunkSize {
				data = data[:read]
			}

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context done %w", context.Cause(ctx))
			case uploadSem <- struct{}{}:
				wg.Add(1)
				go func(i int, chunk []byte) {
					defer func() {
						<-uploadSem
						wg.Done()
					}()

					resp, err := api.uploadChunk(fileUpload, i, data)
					if err != nil {
						cancel(err)
						return
					}
					select { // only care about getting this once
					case bucketAndRegion <- *resp:
					default:
					}
				}(i, data)
			}
		}

		if err == io.EOF {
			break
		}
	}

	if size == 0 {
		return api.completeUploadEmpty(fileUpload)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case resp, ok := <-bucketAndRegion:
			if !ok {
				return nil, fmt.Errorf("no chunks successfully uploaded")
			}
			return api.completeUpload(fileUpload, resp.Bucket, resp.Region, size)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("context done %w", context.Cause(ctx))

	}
}

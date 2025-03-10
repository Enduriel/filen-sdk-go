package filen

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"hash"
	"io"
	"sync"
)

func (api *Filen) fetchAndDecryptChunk(ctx context.Context, file *types.File, chunkIndex int) ([]byte, error) {
	// could potentially be optimized by accepting a []byte buffer to reuse
	encryptedBytes, err := api.client.DownloadFileChunk(ctx, file.UUID, file.Region, file.Bucket, chunkIndex)
	if err != nil {
		return nil, fmt.Errorf("downloading chunk %d: %w", chunkIndex, err)
	}
	decryptedBytes, err := file.EncryptionKey.DecryptData(encryptedBytes)
	if err != nil {
		return nil, fmt.Errorf("decrypting chunk %d: %w", chunkIndex, err)
	}
	return decryptedBytes, nil
}

// Configurable constants
const (
	// MaxBufferSize is the number of chunks to keep in memory
	MaxBufferSize = 8
)

type CtxMutex struct {
	channel chan struct{}
}

func NewCtxMutex() CtxMutex {
	return CtxMutex{
		channel: make(chan struct{}, 1),
	}
}

func (m *CtxMutex) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case m.channel <- struct{}{}:
		return nil
	}
}

func (m *CtxMutex) MustLock() {
	select {
	case m.channel <- struct{}{}:
		return
	default:
		panic("locking locked mutex")
	}
}

func (m *CtxMutex) Unlock() {
	select {
	case <-m.channel:
		return
	default:
		panic("unlocking unlocked mutex")
	}
}

// chunkState represents the state of a single chunk in the buffer
type chunkState struct {
	data  [ChunkSize]byte // Fixed-size array for optimal cache locality
	size  int             // Actual size of data (may be less than ChunkSize for the last chunk)
	ctxMu CtxMutex        // Mutex for this specific chunk
}

func (c *chunkState) copyTo(ctx context.Context, out []byte, offset int, maxLength int) (int, error) {
	err := c.ctxMu.Lock(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to lock CtxMutex: %w", err)
	}
	defer c.ctxMu.Unlock()
	if offset >= c.size {
		return 0, io.EOF
	}

	available := c.size - offset
	if maxLength > available {
		maxLength = available
	}

	copy(out, c.data[offset:offset+maxLength])
	return maxLength, nil
}

// ChunkedReader implements io.Reader for sequential chunked file downloads
type ChunkedReader struct {
	file          *types.File
	api           *Filen
	buffer        []chunkState // Fixed-size circular buffer of chunks
	chunkIndex    int          // Index of the current chunk being read
	offsetInChunk int          // Current offset within the current chunk
	ctx           context.Context
	cancel        context.CancelCauseFunc
	errOnce       *sync.Once
	hasher        hash.Hash
}

// newChunkedReader creates a new ChunkedReader for sequential reading
func newChunkedReader(ctx context.Context, api *Filen, file *types.File) *ChunkedReader {
	ctx, cancel := context.WithCancelCause(ctx)
	bufferSize := min(MaxBufferSize, file.Chunks)

	reader := &ChunkedReader{
		file:          file,
		api:           api,
		buffer:        make([]chunkState, bufferSize),
		chunkIndex:    0,
		offsetInChunk: 0,
		ctx:           ctx,
		cancel:        cancel,
		errOnce:       &sync.Once{},
		hasher:        sha512.New(),
	}

	// Init and prefetch initial chunks
	for i := 0; i < bufferSize; i++ {
		reader.buffer[i].ctxMu = NewCtxMutex()
		reader.goFetchChunk(i)
	}
	return reader
}

func (r *ChunkedReader) fetchChunk(c *chunkState, chunkIndex int) {
	data, err := r.api.fetchAndDecryptChunk(r.ctx, r.file, chunkIndex)
	if err != nil {
		r.errOnce.Do(func() { r.cancel(fmt.Errorf("failed to fetch chunk %d: %w", chunkIndex, err)) })
		return
	}
	if len(data) > ChunkSize {
		r.errOnce.Do(func() { r.cancel(fmt.Errorf("chunk %d is too large: %d bytes", chunkIndex, len(data))) })
		return
	}
	copy(c.data[:], data)
	c.size = len(data)
}

func (r *ChunkedReader) goFetchChunk(chunkIndex int) {
	if chunkIndex >= r.file.Chunks {
		return
	}
	bufferPos := chunkIndex % len(r.buffer)
	chunkState := &r.buffer[bufferPos]
	chunkState.ctxMu.MustLock()
	go func() {
		defer chunkState.ctxMu.Unlock()
		r.fetchChunk(chunkState, chunkIndex)
	}()
}

// Read implements io.Reader - optimized for sequential reading
func (r *ChunkedReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Check for fetch errors
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	read := 0
	for read < len(p) {
		select {
		case <-r.ctx.Done():
			return read, r.ctx.Err()
		default:
			// continue
		}
		// Check if we've reached EOF
		if r.chunkIndex >= r.file.Chunks {
			if read == 0 {
				return 0, io.EOF
			}
			break
		}

		currentChunk := &r.buffer[r.chunkIndex%len(r.buffer)]
		copiedLen, err := currentChunk.copyTo(r.ctx, p[read:], r.offsetInChunk, len(p)-read)
		if err == io.EOF {
			// this shouldn't really happen, but just in case
			r.goFetchChunk(r.chunkIndex + len(r.buffer))
			r.offsetInChunk = 0
			r.chunkIndex++
			continue
		} else if err != nil {
			return read, fmt.Errorf("failed to read chunk: %w", err)
		}
		read += copiedLen
		r.offsetInChunk += copiedLen
		// Check if finished reading chunk
		if r.offsetInChunk >= currentChunk.size {
			r.goFetchChunk(r.chunkIndex + len(r.buffer))
			r.offsetInChunk = 0
			r.chunkIndex++
		}
	}
	r.hasher.Write(p[:read])
	return read, nil
}

// Close cleans up resources used by the reader
func (r *ChunkedReader) Close() error {
	r.cancel(fmt.Errorf("reader closed")) // Cancel all ongoing operations

	if r.chunkIndex < r.file.Chunks {
		// incomplete read
		return nil
	}
	if r.file.Hash != "" {
		h := hex.EncodeToString(r.hasher.Sum(nil))
		if r.file.Hash != h {
			return fmt.Errorf("hash mismatch: expected %s, got %s", r.file.Hash, h)
		}
	}
	// should we be replacing the hash if it's empty?
	return nil
}

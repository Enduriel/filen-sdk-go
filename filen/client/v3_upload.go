package client

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"time"
)

type v3uploadResponse struct {
	Bucket string `json:"bucket"`
	Region string `json:"region"`
}

// PostV3Upload uploads a file chunk to the storage backend.
func (client *Client) PostV3Upload(uuid string, chunkIdx int, parentUUID string, uploadKey string, data []byte) (string, string, error) {
	startTime := time.Now()
	// build request
	fmt.Printf("started uploading chunk %d\n", chunkIdx)
	dataHash := hex.EncodeToString(crypto.RunSHA521(data))
	url := &FilenURL{
		Type: URLTypeIngest,
		Path: fmt.Sprintf("/v3/upload?uuid=%s&index=%v&parent=%s&uploadKey=%s&hash=%s",
			uuid, chunkIdx, parentUUID, uploadKey, dataHash),
	}
	method := "POST"
	// Can't use the standard Client.RequestData because our request body is raw bytes
	req, err := client.buildReaderRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return "", "", err
	}

	response, err := handleRequest(req, &client.httpClient, method, url)
	if err != nil {
		return "", "", err
	}

	if response.Status == false {
		return "", "", errors.New("Cannot upload file chunk: " + response.Message)
	}

	uploadResponse := &v3uploadResponse{}
	err = response.IntoData(uploadResponse)
	if err != nil {
		return "", "", err
	}
	fmt.Printf("time to upload chunk %d: %s\n", chunkIdx, time.Since(startTime))
	return uploadResponse.Region, uploadResponse.Bucket, nil
}

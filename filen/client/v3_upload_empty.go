package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type V3UploadEmptyRequest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	Size       string                 `json:"size"`
	Parent     string                 `json:"parent"`
	MimeType   crypto.EncryptedString `json:"mime"`
	Metadata   crypto.EncryptedString `json:"metadata"`
	Version    int                    `json:"version"`
}

func (c *Client) PostV3UploadEmpty(ctx context.Context, request V3UploadEmptyRequest) (*V3UploadDoneResponse, error) {
	response := &V3UploadDoneResponse{}
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/upload/empty"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

package client

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

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

type V3UploadEmptyResponse struct {
	Chunks int `json:"chunks"`
	Size   int `json:"size"`
}

func (c *Client) PostV3UploadEmpty(request V3UploadEmptyRequest) (*V3UploadEmptyResponse, error) {
	response := &V3UploadEmptyResponse{}
	_, err := c.RequestData("POST", GatewayURL("/v3/upload/empty"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

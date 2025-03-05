package client

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

type V3UploadDoneRequest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	Size       string                 `json:"size"`
	Chunks     int                    `json:"chunks"`
	MimeType   crypto.EncryptedString `json:"mime"`
	Rm         string                 `json:"rm"`
	Metadata   crypto.EncryptedString `json:"metadata"`
	Version    int                    `json:"version"`
	UploadKey  string                 `json:"uploadKey"`
}

type V3UploadDoneResponse struct {
	Chunks int `json:"chunks"`
	Size   int `json:"size"`
}

// PostV3UploadDone calls /v3/upload/done.
func (c *Client) PostV3UploadDone(request V3UploadDoneRequest) (*V3UploadDoneResponse, error) {
	response := &V3UploadDoneResponse{}
	_, err := c.RequestData("POST", GatewayURL("/v3/upload/done"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

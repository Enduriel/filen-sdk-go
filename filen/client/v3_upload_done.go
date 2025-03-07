package client

import (
	"context"
)

type V3UploadDoneRequest struct {
	V3UploadEmptyRequest
	Chunks    int    `json:"chunks"`
	Rm        string `json:"rm"`
	UploadKey string `json:"uploadKey"`
}

type V3UploadDoneResponse struct {
	Chunks int `json:"chunks"`
	Size   int `json:"size"`
}

// PostV3UploadDone calls /v3/upload/done.
func (c *Client) PostV3UploadDone(ctx context.Context, request V3UploadDoneRequest) (*V3UploadDoneResponse, error) {
	response := &V3UploadDoneResponse{}
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/upload/done"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

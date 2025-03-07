package client

import "context"

type V3UserBaseFolderResponse struct {
	UUID string `json:"uuid"`
}

// GetV3UserBaseFolder calls /v3/user/baseFolder.
func (c *Client) GetV3UserBaseFolder(ctx context.Context) (*V3UserBaseFolderResponse, error) {
	userBaseFolder := &V3UserBaseFolderResponse{}
	_, err := c.RequestData(ctx, "GET", GatewayURL("/v3/user/baseFolder"), nil, userBaseFolder)
	return userBaseFolder, err
}

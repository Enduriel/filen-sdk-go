package client

type V3UserBaseFolderResponse struct {
	UUID string `json:"uuid"`
}

// GetV3UserBaseFolder calls /v3/user/baseFolder.
func (client *Client) GetV3UserBaseFolder() (*V3UserBaseFolderResponse, error) {
	userBaseFolder := &V3UserBaseFolderResponse{}
	_, err := client.RequestData("GET", GatewayURL("/v3/user/baseFolder"), nil, userBaseFolder)
	return userBaseFolder, err
}

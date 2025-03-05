package client

type v3fileTrashRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3FileTrash(uuid string) error {
	_, err := c.Request("POST", GatewayURL("/v3/file/trash"), v3fileTrashRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

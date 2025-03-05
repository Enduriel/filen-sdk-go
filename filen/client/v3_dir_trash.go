package client

type v3DirTrashRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3DirTrash(uuid string) error {
	_, err := c.Request("POST", GatewayURL("/v3/dir/trash"), v3DirTrashRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

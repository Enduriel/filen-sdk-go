package client

import "context"

type v3DirTrashRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3DirTrash(ctx context.Context, uuid string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/dir/trash"), v3DirTrashRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

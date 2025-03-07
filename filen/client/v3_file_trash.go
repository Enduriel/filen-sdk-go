package client

import "context"

type v3fileTrashRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3FileTrash(ctx context.Context, uuid string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/file/trash"), v3fileTrashRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

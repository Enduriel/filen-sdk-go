package client

import "context"

type v3DirDeletePermanentRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3DirDeletePermanent(ctx context.Context, uuid string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/dir/delete/permanent"), v3DirDeletePermanentRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

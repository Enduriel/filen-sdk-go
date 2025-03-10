package client

import "context"

type v3fileDeletePermanentRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3FileDeletePermanent(ctx context.Context, uuid string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/file/delete/permanent"), v3fileDeletePermanentRequest{
		UUID: uuid,
	})
	if err != nil {
		return err
	}
	return nil
}

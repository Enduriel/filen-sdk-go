package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3createDirReqeuest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	ParentUUID string                 `json:"parent"`
}

type V3CreateDirResponse struct {
	UUID string `json:"uuid"`
}

// PostV3DirCreate calls /v3/dir/create
func (c *Client) PostV3DirCreate(ctx context.Context, uuid string, name crypto.EncryptedString, nameHashed string, parentUUID string) (*V3CreateDirResponse, error) {
	response := &V3CreateDirResponse{}
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/create"), v3createDirReqeuest{
		UUID:       uuid,
		Name:       name,
		NameHashed: nameHashed,
		ParentUUID: parentUUID,
	}, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

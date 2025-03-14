package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3userMasterKeysRequest struct {
	MasterKey crypto.EncryptedString `json:"masterKeys"`
}

type V3UserMasterKeysResponse struct {
	Keys crypto.EncryptedString `json:"keys"`
}

// PostV3UserMasterKeys calls /v3/user/masterKeys.
func (c *Client) PostV3UserMasterKeys(ctx context.Context, encryptedMasterKey crypto.EncryptedString) (*V3UserMasterKeysResponse, error) {
	userMasterKeys := &V3UserMasterKeysResponse{}
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/user/masterKeys"), v3userMasterKeysRequest{
		MasterKey: encryptedMasterKey,
	}, userMasterKeys)
	return userMasterKeys, err
}

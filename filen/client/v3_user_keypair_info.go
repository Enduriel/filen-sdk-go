package client

import (
	"context"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type V3UserKeyPairInfoResponse struct {
	PrivateKey crypto.EncryptedString `json:"privateKey"`
	PublicKey  string                 `json:"publicKey"`
}

func (c *Client) GetV3UserKeyPairInfo(ctx context.Context) (*V3UserKeyPairInfoResponse, error) {
	response := &V3UserKeyPairInfoResponse{}
	_, err := c.RequestData(ctx, "GET", GatewayURL("/v3/user/keyPair/info"), nil, response)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return response, nil
}

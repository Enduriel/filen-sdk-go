package client

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

type v3userMasterKeysRequest struct {
	MasterKey crypto.EncryptedString `json:"masterKeys"`
}

type V3UserMasterKeysResponse struct {
	Keys crypto.EncryptedString `json:"keys"`
}

// PostV3UserMasterKeys calls /v3/user/masterKeys.
func (client *Client) PostV3UserMasterKeys(encryptedMasterKey crypto.EncryptedString) (*V3UserMasterKeysResponse, error) {
	userMasterKeys := &V3UserMasterKeysResponse{}
	_, err := client.RequestData("POST", GatewayURL("/v3/user/masterKeys"), v3userMasterKeysRequest{
		MasterKey: encryptedMasterKey,
	}, userMasterKeys)
	return userMasterKeys, err
}

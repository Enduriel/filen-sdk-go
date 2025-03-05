package client

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

type v3userDekRequest struct {
	DEK crypto.EncryptedString `json:"dek"`
}

func (c *Client) PostV3UserDEK(encryptedDEK crypto.EncryptedString) error {
	_, err := c.Request("POST", GatewayURL("/v3/user/dek"), v3userDekRequest{
		DEK: encryptedDEK,
	})
	if err != nil {
		return err
	}
	return nil
}

type v3userDEKResponse struct {
	DEK crypto.EncryptedString `json:"dek"`
}

func (c *Client) GetV3UserDEK() (crypto.EncryptedString, error) {
	response := &v3userDEKResponse{}
	_, err := c.RequestData("GET", GatewayURL("/v3/user/dek"), nil, response)
	if err != nil {
		return "", err
	}
	return response.DEK, nil
}

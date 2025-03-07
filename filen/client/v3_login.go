package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3loginRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"twoFactorCode"`
	AuthVersion   int    `json:"authVersion"`
}

type V3LoginResponse struct {
	APIKey     string                 `json:"apiKey"`
	MasterKeys crypto.EncryptedString `json:"masterKeys"`
	PublicKey  string                 `json:"publicKey"`
	PrivateKey crypto.EncryptedString `json:"privateKey"`
	DEK        crypto.EncryptedString `json:"dek"`
}

// PostV3Login calls /v3/login.
func (uc *UnauthorizedClient) PostV3Login(ctx context.Context, email string, password crypto.DerivedPassword) (*V3LoginResponse, error) {
	response := &V3LoginResponse{}
	_, err := uc.RequestData(ctx, "POST", GatewayURL("/v3/login"), v3loginRequest{
		Email:         email,
		Password:      string(password),
		TwoFactorCode: "XXXXXX",
		AuthVersion:   2, // TODO: make this configurable
	}, response)
	return response, err
}

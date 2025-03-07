package client

import "context"

type v3authInfoRequest struct {
	Email string `json:"email"`
}

type V3AuthInfoResponse struct {
	AuthVersion int    `json:"authVersion"`
	Salt        string `json:"salt"`
}

// PostV3AuthInfo calls /v3/auth/info.
func (uc *UnauthorizedClient) PostV3AuthInfo(ctx context.Context, email string) (*V3AuthInfoResponse, error) {
	authInfo := &V3AuthInfoResponse{}
	_, err := uc.RequestData(ctx, "POST", GatewayURL("/v3/auth/info"), v3authInfoRequest{
		Email: email,
	}, authInfo)
	return authInfo, err
}

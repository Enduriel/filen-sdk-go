package client

import (
	"context"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3FileMetadataRequest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	Metadata   crypto.EncryptedString `json:"metadata"`
}

func (c *Client) PostV3FileMetadata(ctx context.Context, uuid string, name crypto.EncryptedString, nameHashed string, metadata crypto.EncryptedString) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/file/metadata"), v3FileMetadataRequest{
		UUID:       uuid,
		Name:       name,
		NameHashed: nameHashed,
		Metadata:   metadata,
	})
	if err != nil {
		return fmt.Errorf("post v3 file metadata: %w", err)
	}
	return nil
}

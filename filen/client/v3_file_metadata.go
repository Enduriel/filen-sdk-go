package client

import (
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3FileMetadataRequest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	Metadata   crypto.EncryptedString `json:"metadata"`
}

func (client *Client) PostV3FileMetadata(uuid string, name crypto.EncryptedString, nameHashed string, metadata crypto.EncryptedString) error {
	_, err := client.Request("POST", GatewayURL("/v3/file/metadata"), v3FileMetadataRequest{
		UUID:       uuid,
		Name:       name,
		NameHashed: nameHashed,
		Metadata:   metadata,
	})
	return fmt.Errorf("post v3 file metadata: %w", err)
}

package client

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

type v3dirContentRequest struct {
	UUID string `json:"uuid"`
}

type V3DirContentResponse struct {
	Uploads []struct {
		UUID      string                 `json:"uuid"`
		Metadata  crypto.EncryptedString `json:"metadata"`
		Rm        string                 `json:"rm"`
		Timestamp int                    `json:"timestamp"`
		Chunks    int                    `json:"chunks"`
		Size      int                    `json:"size"`
		Bucket    string                 `json:"bucket"`
		Region    string                 `json:"region"`
		Parent    string                 `json:"parent"`
		Version   int                    `json:"version"`
		Favorited int                    `json:"favorited"`
	} `json:"uploads"`
	Folders []struct {
		UUID      string                 `json:"uuid"`
		Name      crypto.EncryptedString `json:"name"`
		Parent    string                 `json:"parent"`
		Color     interface{}            `json:"color"`
		Timestamp int                    `json:"timestamp"`
		Favorited int                    `json:"favorited"`
		IsSync    int                    `json:"is_sync"`
		IsDefault int                    `json:"is_default"`
	} `json:"folders"`
}

// PostV3DirContent calls /v3/dir/content.
func (client *Client) PostV3DirContent(uuid string) (*V3DirContentResponse, error) {
	directoryContent := &V3DirContentResponse{}
	_, err := client.RequestData("POST", GatewayURL("/v3/dir/content"), v3dirContentRequest{
		UUID: uuid,
	}, directoryContent)
	return directoryContent, err
}

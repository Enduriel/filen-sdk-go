package client

import (
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

// /v3/auth/info

type AuthInfo struct {
	AuthVersion int    `json:"authVersion"`
	Salt        string `json:"salt"`
}

// GetAuthInfo calls /v3/auth/info.
func (client *UnauthorizedClient) GetAuthInfo(email string) (*AuthInfo, error) {
	request := struct {
		Email string `json:"email"`
	}{email}
	authInfo := &AuthInfo{}
	_, err := client.RequestData("POST", GatewayURL("/v3/auth/info"), request, authInfo)
	return authInfo, err
}

// /v3/login

type LoginResponse struct {
	APIKey     string                 `json:"apiKey"`
	MasterKeys crypto.EncryptedString `json:"masterKeys"`
	PublicKey  string                 `json:"publicKey"`
	PrivateKey crypto.EncryptedString `json:"privateKey"`
	DEK        crypto.EncryptedString `json:"dek"`
}

// Login calls /v3/login.
func (client *UnauthorizedClient) Login(email string, password crypto.DerivedPassword) (*LoginResponse, error) {
	request := struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		TwoFactorCode string `json:"twoFactorCode"`
		AuthVersion   int    `json:"authVersion"`
	}{email, string(password), "XXXXXX", 2}
	response := &LoginResponse{}
	_, err := client.RequestData("POST", GatewayURL("/v3/login"), request, response)
	return response, err
}

// /v3/user/baseFolder

type UserBaseFolder struct {
	UUID string `json:"uuid"`
}

// GetUserBaseFolder calls /v3/user/baseFolder.
func (client *Client) GetUserBaseFolder() (*UserBaseFolder, error) {
	userBaseFolder := &UserBaseFolder{}
	_, err := client.RequestData("GET", GatewayURL("/v3/user/baseFolder"), nil, userBaseFolder)
	return userBaseFolder, err
}

// /v3/dir/content

type DirectoryContent struct {
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

// GetDirectoryContent calls /v3/dir/content.
func (client *Client) GetDirectoryContent(uuid string) (*DirectoryContent, error) {
	request := struct {
		UUID string `json:"uuid"`
	}{uuid}
	directoryContent := &DirectoryContent{}
	_, err := client.RequestData("POST", GatewayURL("/v3/dir/content"), request, directoryContent)
	return directoryContent, err
}

// /v3/user/masterKeys

type UserMasterKeys struct {
	Keys crypto.EncryptedString `json:"keys"`
}

// GetUserMasterKeys calls /v3/user/masterKeys.
func (client *Client) GetUserMasterKeys(encryptedMasterKey crypto.EncryptedString) (*UserMasterKeys, error) {
	request := struct {
		MasterKey crypto.EncryptedString `json:"masterKeys"`
	}{encryptedMasterKey}
	userMasterKeys := &UserMasterKeys{}
	_, err := client.RequestData("POST", GatewayURL("/v3/user/masterKeys"), request, userMasterKeys)
	return userMasterKeys, err
}

// /v3/upload/done

type UploadDoneRequest struct {
	UUID       string                 `json:"uuid"`
	Name       crypto.EncryptedString `json:"name"`
	NameHashed string                 `json:"nameHashed"`
	Size       string                 `json:"size"`
	Chunks     int                    `json:"chunks"`
	MimeType   crypto.EncryptedString `json:"mime"`
	Rm         string                 `json:"rm"`
	Metadata   crypto.EncryptedString `json:"metadata"`
	Version    int                    `json:"version"`
	UploadKey  string                 `json:"uploadKey"`
}

type UploadDoneResponse struct {
	Chunks int `json:"chunks"`
	Size   int `json:"size"`
}

// UploadDone calls /v3/upload/done.
func (client *Client) UploadDone(request UploadDoneRequest) (*UploadDoneResponse, error) {
	response := &UploadDoneResponse{}
	_, err := client.RequestData("POST", GatewayURL("/v3/upload/done"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// /v3/file/trash

// TrashFile calls /v3/file/trash
func (client *Client) TrashFile(uuid string) error {
	request := struct {
		UUID string `json:"uuid"`
	}{uuid}
	_, err := client.Request("POST", GatewayURL("/v3/file/trash"), request)
	if err != nil {
		return err
	}
	return nil
}

// /v3/dir/create

type CreateDirectoryResponse struct {
	UUID string `json:"uuid"`
}

// CreateDirectory calls /v3/dir/create
func (client *Client) CreateDirectory(uuid string, name crypto.EncryptedString, nameHashed string, parentUUID string) (*CreateDirectoryResponse, error) {
	request := struct {
		UUID       string                 `json:"uuid"`
		Name       crypto.EncryptedString `json:"name"`
		NameHashed string                 `json:"nameHashed"`
		ParentUUID string                 `json:"parent"`
	}{uuid, name, nameHashed, parentUUID}
	response := &CreateDirectoryResponse{}
	_, err := client.RequestData("POST", GatewayURL("/v3/dir/create"), request, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// /v3/dir/trash

// TrashDirectory calls /v3/dir/trash
func (client *Client) TrashDirectory(uuid string) error {
	request := struct {
		UUID string `json:"uuid"`
	}{uuid}
	_, err := client.Request("POST", GatewayURL("/v3/dir/trash"), request)
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) PostV3UserDEK(encryptedDEK crypto.EncryptedString) error {
	request := struct {
		DEK crypto.EncryptedString `json:"dek"`
	}{encryptedDEK}
	_, err := client.Request("POST", GatewayURL("/v3/user/dek"), request)
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) GetV3UserDEK() (crypto.EncryptedString, error) {
	var response struct {
		DEK crypto.EncryptedString `json:"dek"`
	}
	_, err := client.RequestData("GET", GatewayURL("/v3/user/dek"), nil, &response)
	if err != nil {
		return "", err
	}
	return response.DEK, nil
}

// Package client handles HTTP requests to the API and storage backends.
//
// API definitions are at https://gateway.filen.io/v3/docs.
package client

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var (
	gatewayURLs = []string{
		"https://gateway.filen.io",
		/*"https://gateway.filen.net",
		"https://gateway.filen-1.net",
		"https://gateway.filen-2.net",
		"https://gateway.filen-3.net",
		"https://gateway.filen-4.net",
		"https://gateway.filen-5.net",
		"https://gateway.filen-6.net",*/
	}
	egestURLs = []string{
		"https://egest.filen.io",
		/*"https://egest.filen.net",
		"https://egest.filen-1.net",
		"https://egest.filen-2.net",
		"https://egest.filen-3.net",
		"https://egest.filen-4.net",
		"https://egest.filen-5.net",
		"https://egest.filen-6.net",*/
	}
	ingestURLs = []string{
		"https://ingest.filen.io",
		/*"https://ingest.filen.net",
		"https://ingest.filen-1.net",
		"https://ingest.filen-2.net",
		"https://ingest.filen-3.net",
		"https://ingest.filen-4.net",
		"https://ingest.filen-5.net",
		"https://ingest.filen-6.net",*/
	}
)

const (
	URLTypeIngest  = 1
	URLTypeEgest   = 2
	URLTypeGateway = 3
)

type FilenURL struct {
	Type      int
	Path      string
	CachedUrl string
}

func GatewayURL(path string) *FilenURL {
	return &FilenURL{
		Type:      URLTypeGateway,
		Path:      path,
		CachedUrl: "",
	}
}

func (url *FilenURL) String() string {
	if url.CachedUrl == "" {
		var builder strings.Builder
		switch url.Type {
		case URLTypeIngest:
			builder.WriteString(ingestURLs[rand.Intn(len(ingestURLs))])
		case URLTypeEgest:
			builder.WriteString(egestURLs[rand.Intn(len(egestURLs))])
		case URLTypeGateway:
			builder.WriteString(gatewayURLs[rand.Intn(len(gatewayURLs))])
		}
		builder.WriteString(url.Path)
		url.CachedUrl = builder.String()
	}

	return url.CachedUrl
}

type UnauthorizedClient struct {
	httpClient http.Client // cached request client
}

type Client struct {
	UnauthorizedClient
	APIKey string // the Filen API key
}

func New() *UnauthorizedClient {
	return &UnauthorizedClient{
		httpClient: http.Client{Timeout: 10 * time.Second},
	}
}

func (client *UnauthorizedClient) Authorize(apiKey string) *Client {
	return &Client{
		UnauthorizedClient: *client,
		APIKey:             apiKey,
	}
}

// A RequestError carries information on a failed HTTP request.
type RequestError struct {
	Message         string    // description of where the error occurred
	Method          string    // HTTP method of the request
	URL             *FilenURL // URL path of the request
	UnderlyingError error     // the underlying error
}

func (e *RequestError) Error() string {
	var builder strings.Builder
	builder.WriteString(e.Method)
	builder.WriteRune(' ')
	if e.URL.CachedUrl != "" {
		builder.WriteString(fmt.Sprintf("cached: %s", e.URL.CachedUrl))
	} else {
		builder.WriteString(e.URL.Path)
	}
	builder.WriteString(fmt.Sprintf(": %s", e.Message))
	if e.UnderlyingError != nil {
		builder.WriteString(fmt.Sprintf(" (%s)", e.UnderlyingError))
	}
	return builder.String()
}

// cannotSendError returns a RequestError from an error that occurred while sending an HTTP request.
func cannotSendError(method string, url *FilenURL, err error) error {
	return &RequestError{
		Message:         "Cannot send request",
		Method:          method,
		URL:             url,
		UnderlyingError: err,
	}
}

func (client *UnauthorizedClient) buildReaderRequest(method string, url *FilenURL, data io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url.String(), data)
	if err != nil {
		return nil, &RequestError{"Cannot build requestData", method, url, err}
	}
	return req, nil
}

func (client *Client) buildReaderRequest(method string, url *FilenURL, data io.Reader) (*http.Request, error) {
	var request, err = client.UnauthorizedClient.buildReaderRequest(method, url, data)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.APIKey)
	return request, nil
}

func (client *UnauthorizedClient) buildJSONRequest(method string, url *FilenURL, requestData any) (*http.Request, error) {
	var marshalled []byte
	if requestData != nil {
		var err error
		marshalled, err = json.Marshal(requestData)
		if err != nil {
			return nil, &RequestError{fmt.Sprintf("Cannot unmarshal requestData body %#v", requestData), method, url, err}
		}
	}
	req, err := client.buildReaderRequest(method, url, bytes.NewReader(marshalled))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (client *Client) buildJSONRequest(method string, url *FilenURL, requestData any) (*http.Request, error) {
	var request, err = client.UnauthorizedClient.buildJSONRequest(method, url, requestData)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.APIKey)
	return request, nil
}

// parseResponse reads and unmarshals an HTTP response body into an APIResponse.
// It takes the HTTP method, path, and response as arguments.
// If the response body cannot be read or unmarshalled, it returns a RequestError.
// Otherwise, it returns the parsed APIResponse.
func parseResponse(method string, url *FilenURL, response *http.Response) (*APIResponse, error) {
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &RequestError{"Cannot read response body", method, url, err}
	}
	apiResponse := APIResponse{}
	err = json.Unmarshal(resBody, &apiResponse)
	if err != nil {
		return nil, &RequestError{fmt.Sprintf("Cannot unmarshal response %s", string(resBody)), method, url, nil}
	}
	return &apiResponse, nil
}

// handleRequest sends an HTTP request and processes the response.
// It takes a http.Request object, the associated http.Client, and the method and path
// as parameters. It returns an APIResponse containing the parsed response data, or a
// RequestError if the request fails or the response cannot be parsed.
func handleRequest(request *http.Request, httpClient *http.Client, method string, url *FilenURL) (*APIResponse, error) {
	//startTime := time.Now()
	res, err := httpClient.Do(request)
	if err != nil {
		return nil, cannotSendError(method, url, err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	apiRes, err := parseResponse(method, url, res)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Request %s %s took %s\n", method, url, time.Since(startTime))
	return apiRes, nil
}

// convertIntoResponseData unmarshals the response data into the provided output data structure.
// It returns a RequestError if the unmarshalling process fails.
func convertIntoResponseData(method string, url *FilenURL, response *APIResponse, outData any) error {
	err := response.IntoData(outData)
	if err != nil {
		return &RequestError{
			Message:         fmt.Sprintf("Cannot unmarshal response data %#v", response.Data),
			Method:          method,
			URL:             url,
			UnderlyingError: err,
		}
	}
	return nil
}

func (client *UnauthorizedClient) Request(method string, url *FilenURL, requestData any) (*APIResponse, error) {
	request, err := client.buildJSONRequest(method, url, requestData)
	if err != nil {
		return nil, err
	}
	return handleRequest(request, &client.httpClient, method, url)
}

func (client *UnauthorizedClient) RequestData(method string, url *FilenURL, requestData any, outData any) (*APIResponse, error) {
	response, err := client.Request(method, url, requestData)
	if err != nil {
		return nil, err
	}
	err = convertIntoResponseData(method, url, response, outData)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (client *Client) Request(method string, url *FilenURL, requestData any) (*APIResponse, error) {
	request, err := client.buildJSONRequest(method, url, requestData)
	if err != nil {
		return nil, err
	}
	return handleRequest(request, &client.httpClient, method, url)
}

func (client *Client) RequestData(method string, url *FilenURL, requestData any, outData any) (*APIResponse, error) {
	response, err := client.Request(method, url, requestData)
	if err != nil {
		return nil, err
	}
	err = convertIntoResponseData(method, url, response, outData)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// api

// APIResponse represents a response from the API.
type APIResponse struct {
	Status  bool            `json:"status"`  // whether the request was successful
	Message string          `json:"message"` // additional information
	Code    string          `json:"code"`    // a status code
	Data    json.RawMessage `json:"data"`    // response body, or nil
}

func (res *APIResponse) String() string {
	return fmt.Sprintf("ApiResponse{status: %t, message: %s, code: %s, data: %s}", res.Status, res.Message, res.Code, res.Data)
}

// IntoData unmarshals the response body into the provided data structure.
//
// If the response does not contain a body, an error is returned.
// If the unmarshalling process fails, the error is returned.
func (res *APIResponse) IntoData(data any) error {
	if res.Data == nil {
		return errors.New(fmt.Sprintf("No data in response %s", res))
	}
	err := json.Unmarshal(res.Data, data)
	if err != nil {
		return err
	}
	return nil
}

// file chunks

// DownloadFileChunk downloads a file chunk from the storage backend.
func (client *Client) DownloadFileChunk(uuid string, region string, bucket string, chunkIdx int) ([]byte, error) {
	url := &FilenURL{
		Type: URLTypeEgest,
		Path: fmt.Sprintf("/%s/%s/%s/%v", region, bucket, uuid, chunkIdx),
	}

	// Can't use the standard Client.RequestData because the response body is raw bytes
	request, err := client.buildJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.httpClient.Do(request)
	if err != nil {
		return nil, cannotSendError("GET", url, err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// UploadFileChunk uploads a file chunk to the storage backend.
func (client *Client) UploadFileChunk(uuid string, chunkIdx int, parentUUID string, uploadKey string, data []byte) (string, string, error) {
	startTime := time.Now()
	// build request
	fmt.Printf("started uploading chunk %d\n", chunkIdx)
	dataHash := hex.EncodeToString(crypto.RunSHA521(data))
	url := &FilenURL{
		Type: URLTypeIngest,
		Path: fmt.Sprintf("/v3/upload?uuid=%s&index=%v&parent=%s&uploadKey=%s&hash=%s",
			uuid, chunkIdx, parentUUID, uploadKey, dataHash),
	}
	method := "POST"
	// Can't use the standard Client.RequestData because our request body is raw bytes
	req, err := client.buildReaderRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return "", "", err
	}

	response, err := handleRequest(req, &client.httpClient, method, url)
	if err != nil {
		return "", "", err
	}

	if response.Status == false {
		return "", "", errors.New("Cannot upload file chunk: " + response.Message)
	}

	// read response data
	type UploadChunkResponse struct {
		Bucket string `json:"bucket"`
		Region string `json:"region"`
	}
	uploadChunkResponse := &UploadChunkResponse{}
	err = response.IntoData(uploadChunkResponse)
	if err != nil {
		return "", "", err
	}
	fmt.Printf("time to upload chunk %d: %s\n", chunkIdx, time.Since(startTime))
	return uploadChunkResponse.Region, uploadChunkResponse.Bucket, nil
}

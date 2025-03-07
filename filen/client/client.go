// Package client handles HTTP requests to the API and storage backends.
//
// API definitions are at https://gateway.filen.io/v3/docs.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

func (uc *UnauthorizedClient) Authorize(apiKey string) *Client {
	return &Client{
		UnauthorizedClient: *uc,
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

func (uc *UnauthorizedClient) buildReaderRequest(ctx context.Context, method string, url *FilenURL, data io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url.String(), data)
	if err != nil {
		return nil, &RequestError{"Cannot build requestData", method, url, err}
	}
	return req, nil
}

func (c *Client) buildReaderRequest(ctx context.Context, method string, url *FilenURL, data io.Reader) (*http.Request, error) {
	var request, err = c.UnauthorizedClient.buildReaderRequest(ctx, method, url, data)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	return request, nil
}

func (uc *UnauthorizedClient) buildJSONRequest(ctx context.Context, method string, url *FilenURL, requestData any) (*http.Request, error) {
	var marshalled []byte
	if requestData != nil {
		var err error
		marshalled, err = json.Marshal(requestData)
		if err != nil {
			return nil, &RequestError{fmt.Sprintf("Cannot unmarshal requestData body %#v", requestData), method, url, err}
		}
	}
	req, err := uc.buildReaderRequest(ctx, method, url, bytes.NewReader(marshalled))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *Client) buildJSONRequest(ctx context.Context, method string, url *FilenURL, requestData any) (*http.Request, error) {
	var request, err = c.UnauthorizedClient.buildJSONRequest(ctx, method, url, requestData)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	return request, nil
}

// parseResponse reads and unmarshals an HTTP response body into an aPIResponse.
// It takes the HTTP method, path, and response as arguments.
// If the response body cannot be read or unmarshalled, it returns a RequestError.
// Otherwise, it returns the parsed aPIResponse.
func parseResponse(method string, url *FilenURL, response *http.Response) (*aPIResponse, error) {
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, &RequestError{"Cannot read response body", method, url, err}
	}
	apiResponse := aPIResponse{}
	err = json.Unmarshal(resBody, &apiResponse)
	if err != nil {
		return nil, &RequestError{fmt.Sprintf("Cannot unmarshal response %s", string(resBody)), method, url, nil}
	}
	return &apiResponse, nil
}

// handleRequest sends an HTTP request and processes the response.
// It takes a http.Request object, the associated http.Client, and the method and path
// as parameters. It returns an aPIResponse containing the parsed response data, or a
// RequestError if the request fails or the response cannot be parsed.
func handleRequest(request *http.Request, httpClient *http.Client, method string, url *FilenURL) (*aPIResponse, error) {
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
	err = apiRes.CheckError()
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Request %s %s took %s\n", method, url, time.Since(startTime))
	return apiRes, nil
}

// convertIntoResponseData unmarshals the response data into the provided output data structure.
// It returns a RequestError if the unmarshalling process fails.
func convertIntoResponseData(method string, url *FilenURL, response *aPIResponse, outData any) error {
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

func (uc *UnauthorizedClient) Request(ctx context.Context, method string, url *FilenURL, requestData any) (*aPIResponse, error) {
	request, err := uc.buildJSONRequest(ctx, method, url, requestData)
	if err != nil {
		return nil, err
	}
	return handleRequest(request, &uc.httpClient, method, url)
}

func (uc *UnauthorizedClient) RequestData(ctx context.Context, method string, url *FilenURL, requestData any, outData any) (*aPIResponse, error) {
	response, err := uc.Request(ctx, method, url, requestData)
	if err != nil {
		return nil, err
	}
	err = convertIntoResponseData(method, url, response, outData)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) Request(ctx context.Context, method string, url *FilenURL, requestData any) (*aPIResponse, error) {
	request, err := c.buildJSONRequest(ctx, method, url, requestData)
	if err != nil {
		return nil, err
	}
	return handleRequest(request, &c.httpClient, method, url)
}

func (c *Client) RequestData(ctx context.Context, method string, url *FilenURL, requestData any, outData any) (*aPIResponse, error) {
	response, err := c.Request(ctx, method, url, requestData)
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

// aPIResponse represents a response from the API.
type aPIResponse struct {
	Status  bool            `json:"status"`  // whether the request was successful
	Message string          `json:"message"` // additional information
	Code    string          `json:"code"`    // a status code
	Data    json.RawMessage `json:"data"`    // response body, or nil
}

func (res *aPIResponse) CheckError() error {
	if !res.Status {
		return fmt.Errorf("response error: %s %s", res.Message, res.Code)
	}
	return nil
}

func (res *aPIResponse) String() string {
	return fmt.Sprintf("ApiResponse{status: %t, message: %s, code: %s, data: %s}", res.Status, res.Message, res.Code, res.Data)
}

// IntoData unmarshals the response body into the provided data structure.
//
// If the response does not contain a body, an error is returned.
// If the unmarshalling process fails, the error is returned.
func (res *aPIResponse) IntoData(data any) error {
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
func (c *Client) DownloadFileChunk(ctx context.Context, uuid string, region string, bucket string, chunkIdx int) ([]byte, error) {
	url := &FilenURL{
		Type: URLTypeEgest,
		Path: fmt.Sprintf("/%s/%s/%s/%v", region, bucket, uuid, chunkIdx),
	}

	// Can't use the standard Client.RequestData because the response body is raw bytes
	request, err := c.buildJSONRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(request)
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

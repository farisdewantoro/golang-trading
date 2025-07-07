package httpclient

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
)

type RestyClient struct {
	client *resty.Client
}

func New(baseURL string, timeout time.Duration, bearerToken string) HTTPClient {
	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(timeout).
		SetHeader("Accept", "application/json").
		SetAuthToken(bearerToken)

	return &RestyClient{client: client}
}

// GET request with optional query params
func (rc *RestyClient) Get(ctx context.Context, endpoint string, queryParams map[string]string, headers map[string]string, result interface{}) (*BaseResponse, error) {
	req := rc.client.R().SetContext(ctx).SetResult(result)

	if queryParams != nil {
		req.SetQueryParams(queryParams)
	}

	if headers != nil {
		req.SetHeaders(headers)
	}

	resp, err := req.Get(endpoint)
	return &BaseResponse{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Headers:    resp.Header(),
	}, err
}

// POST request with body
func (rc *RestyClient) Post(ctx context.Context, endpoint string, body interface{}, headers map[string]string, result interface{}) (*BaseResponse, error) {

	req := rc.client.R().
		SetContext(ctx).
		SetBody(body).
		SetResult(result)

	if headers != nil {
		req.SetHeaders(headers)
	}

	resp, err := req.Post(endpoint)
	return &BaseResponse{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Headers:    resp.Header(),
	}, err
}

// PUT request
func (rc *RestyClient) Put(ctx context.Context, endpoint string, body interface{}, headers map[string]string, result interface{}) (*BaseResponse, error) {
	req := rc.client.R().
		SetContext(ctx).
		SetBody(body).
		SetResult(result)

	if headers != nil {
		req.SetHeaders(headers)
	}

	resp, err := req.Put(endpoint)
	return &BaseResponse{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Headers:    resp.Header(),
	}, err
}

// DELETE request
func (rc *RestyClient) Delete(ctx context.Context, endpoint string, headers map[string]string, result interface{}) (*BaseResponse, error) {
	req := rc.client.R().
		SetContext(ctx).
		SetResult(result)

	if headers != nil {
		req.SetHeaders(headers)
	}

	resp, err := req.Delete(endpoint)
	return &BaseResponse{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Headers:    resp.Header(),
	}, err
}

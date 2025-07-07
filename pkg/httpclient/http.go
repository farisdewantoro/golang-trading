package httpclient

import (
	"context"
	"net/http"
)

type BaseResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

type HTTPClient interface {
	Get(ctx context.Context, endpoint string, queryParams map[string]string, headers map[string]string, result interface{}) (*BaseResponse, error)
	Post(ctx context.Context, endpoint string, body interface{}, headers map[string]string, result interface{}) (*BaseResponse, error)
	Put(ctx context.Context, endpoint string, body interface{}, headers map[string]string, result interface{}) (*BaseResponse, error)
	Delete(ctx context.Context, endpoint string, headers map[string]string, result interface{}) (*BaseResponse, error)
}

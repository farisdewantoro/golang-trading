package dto

import "net/http"

type BaseResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewBaseResponse(code int, message string, data interface{}) *BaseResponse {
	return &BaseResponse{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func NewBadRequestResponse(message string) *BaseResponse {
	return NewBaseResponse(http.StatusBadRequest, message, nil)
}

func NewSuccessResponse(message string, data interface{}) *BaseResponse {
	return NewBaseResponse(http.StatusOK, message, data)
}

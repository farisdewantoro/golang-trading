package http

import (
	"context"
	"golang-trading/internal/service"

	goValidator "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type HttpAPIHandler struct {
	echo      *echo.Echo
	validator *goValidator.Validate
	service   *service.Service
}

func NewHttpAPIHandler(ctx context.Context, echo *echo.Echo, validator *goValidator.Validate, service *service.Service) *HttpAPIHandler {
	return &HttpAPIHandler{
		echo:      echo,
		validator: validator,
		service:   service,
	}
}

func (h *HttpAPIHandler) SetupRoutes() {
	base := h.echo.Group("/api")
	h.SetupJobs(base)
	h.SetupBacktest(base)
}

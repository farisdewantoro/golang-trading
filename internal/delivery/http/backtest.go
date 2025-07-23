package http

import (
	"golang-trading/internal/dto"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *HttpAPIHandler) SetupBacktest(base *echo.Group) {
	backtestGroup := base.Group("/backtest")
	backtestGroup.POST("", h.runBacktest)
}

func (h *HttpAPIHandler) runBacktest(c echo.Context) error {
	ctx := c.Request().Context()

	req := new(dto.BacktestRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := h.validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	result, err := h.service.BacktestService.RunBacktest(ctx, *req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to run backtest"})
	}

	return c.JSON(http.StatusOK, result)
}

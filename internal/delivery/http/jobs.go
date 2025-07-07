package http

import (
	"golang-trading/internal/dto"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *HttpAPIHandler) SetupJobs(base *echo.Group) {
	v1 := base.Group("/v1/jobs")
	{
		v1.POST("/run", h.RunJobs)
	}

}

func (h *HttpAPIHandler) RunJobs(c echo.Context) error {
	response := dto.NewBaseResponse(http.StatusOK, "Start running jobs", nil)
	if err := h.service.SchedulerService.Execute(c.Request().Context()); err != nil {
		response.Code = http.StatusInternalServerError
		response.Message = err.Error()
	}
	return c.JSON(response.Code, response)
}

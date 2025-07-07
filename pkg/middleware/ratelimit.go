package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// Response represents the error response structure
type Response struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func NewRateLimiterMiddleware() echo.MiddlewareFunc {
	config := middleware.RateLimiterConfig{
		Skipper: middleware.DefaultSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				// Rate: 10 requests per second
				Rate: rate.Limit(10),
				// Burst: Allow up to 30 requests in a burst
				Burst: 30,
				// ExpiresIn: Rate limit state expires after 3 minutes of inactivity
				ExpiresIn: 3 * time.Minute,
			},
		),

		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			id := ctx.RealIP()
			return id, nil
		},

		ErrorHandler: func(context echo.Context, err error) error {
			return context.JSON(http.StatusForbidden, Response{
				Status:  http.StatusForbidden,
				Message: "Access forbidden: Rate limiter error occurred",
			})
		},

		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(http.StatusTooManyRequests, Response{
				Status:  http.StatusTooManyRequests,
				Message: "Too many requests: Rate limit exceeded. Please try again later",
			})
		},
	}

	return middleware.RateLimiterWithConfig(config)
}

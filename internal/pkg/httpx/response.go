package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: 0, Data: data})
}

func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Envelope{Code: 0, Message: msg, Data: data})
}

func Err(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Envelope{Code: code, Message: msg})
}

func BadRequest(c *gin.Context, msg string) {
	Err(c, http.StatusBadRequest, 40000, msg)
}

func Unauthorized(c *gin.Context, msg string) {
	Err(c, http.StatusUnauthorized, 40100, msg)
}

func Forbidden(c *gin.Context, msg string) {
	Err(c, http.StatusForbidden, 40300, msg)
}

func TooManyRequests(c *gin.Context, msg string) {
	Err(c, http.StatusTooManyRequests, 42900, msg)
}

func ServiceUnavailable(c *gin.Context, msg string) {
	Err(c, http.StatusServiceUnavailable, 50300, msg)
}

func ServerError(c *gin.Context, msg string) {
	Err(c, http.StatusInternalServerError, 50000, msg)
}

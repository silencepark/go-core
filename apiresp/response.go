// Package apiresp 统一 JSON 响应格式。
package apiresp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/silencepark/go-core/errs"
)

// Response 统一响应信封。
type Response struct {
	Code       int         `json:"code"`
	Message    string      `json:"message"`
	Data       any         `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination 分页元数据。
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// httpStatus 从业务码推导 HTTP status。
// code==0（成功）返回 200；非 0 委托 errs.Error.HTTPStatus() 统一映射。
func httpStatus(code int) int {
	if code == 0 {
		return http.StatusOK
	}
	return (&errs.Error{Code: code}).HTTPStatus()
}

// JSON 输出统一 JSON 响应。
func JSON(c *gin.Context, code int, message string, data any, pagination *Pagination) {
	c.JSON(httpStatus(code), Response{
		Code:       code,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}

// Success 返回成功响应（code=0）。
func Success(c *gin.Context, data any) {
	JSON(c, 0, "success", data, nil)
}

// SuccessWithPage 返回带分页的成功响应。
func SuccessWithPage(c *gin.Context, data any, page Pagination) {
	JSON(c, 0, "success", data, &page)
}

// FromError 根据 error 类型智能返回响应。
//   - *errs.Error → 自动映射业务码和 HTTP status
//   - 普通 error   → 500，使用通用消息避免泄露内部错误详情
func FromError(c *gin.Context, err error) {
	var be *errs.Error
	if errors.As(err, &be) {
		JSON(c, be.Code, be.Message, nil, nil)
		return
	}
	// 非业务错误不暴露原始消息，内部细节由中间件日志记录
	JSON(c, errs.ErrInternal.Code, errs.ErrInternal.Message, nil, nil)
}

// BadRequest 40000 参数错误。
func BadRequest(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrBadRequest.Message
	}
	JSON(c, errs.ErrBadRequest.Code, message, nil, nil)
}

// Unauthorized 40100 未认证。
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrUnauthorized.Message
	}
	JSON(c, errs.ErrUnauthorized.Code, message, nil, nil)
}

// Forbidden 40300 无权限。
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrForbidden.Message
	}
	JSON(c, errs.ErrForbidden.Code, message, nil, nil)
}

// NotFound 40400 资源不存在。
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrNotFound.Message
	}
	JSON(c, errs.ErrNotFound.Code, message, nil, nil)
}

// Conflict 40900 冲突。
func Conflict(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrConflict.Message
	}
	JSON(c, errs.ErrConflict.Code, message, nil, nil)
}

// InternalServerError 50000 服务器内部错误。
func InternalServerError(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrInternal.Message
	}
	JSON(c, errs.ErrInternal.Code, message, nil, nil)
}

// ServiceUnavailable 50300 服务不可用。
func ServiceUnavailable(c *gin.Context, message string) {
	if message == "" {
		message = errs.ErrServiceUnavailable.Message
	}
	JSON(c, errs.ErrServiceUnavailable.Code, message, nil, nil)
}

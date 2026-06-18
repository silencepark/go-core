// Package errs 定义统一业务错误码体系。
// 业务错误码与 HTTP status code 解耦，语义码范围按业务域划分。
package errs

import (
	"errors"
	"fmt"
)

// Error 业务错误，携带语义码、用户消息及底层原始错误。
type Error struct {
	Code    int    // 业务错误码（5 位，前 3 位对应 HTTP 语义段）
	Message string // 面向用户的提示信息
	err     error  // 底层 wrapped error（内部日志用，不对外暴露）
}

// New 创建业务错误。
func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap 包装底层错误，附加业务码与消息。
func Wrap(code int, message string, err error) *Error {
	return &Error{Code: code, Message: message, err: err}
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap，支持 errors.Is/As。
func (e *Error) Unwrap() error { return e.err }

// HTTPStatus 将业务码映射回 HTTP 状态码。
// 规则：code/100 作为 HTTP status（如 40401 → 404）。
func (e *Error) HTTPStatus() int {
	if e.Code >= 40000 && e.Code < 60000 {
		return e.Code / 100
	}
	return 500
}

// UnwrapMessage 递归获取最内层非业务错误的原始消息。
func (e *Error) UnwrapMessage() string {
	inner := e
	for {
		if inner.err == nil {
			return inner.Message
		}
		if be, ok := inner.err.(*Error); ok {
			inner = be
			continue
		}
		return inner.Message
	}
}

// IsDuplicateKey 判断 error 链中是否包含 MySQL 唯一键冲突（Error 1062）。
func IsDuplicateKey(err error) bool {
	var mysqlErr interface{ Number() uint16 }
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number() == 1062
	}
	return false
}

// ── 通用错误码 ──────────────────────────────────────

// 400xx — 请求/参数类错误
var (
	ErrBadRequest   = New(40000, "bad request")
	ErrParamInvalid = New(40001, "invalid parameter")
)

// 401xx — 认证错误
var (
	ErrUnauthorized       = New(40100, "unauthorized")
	ErrTokenMissing       = New(40101, "missing authorization header")
	ErrTokenInvalid       = New(40102, "invalid or expired token")
	ErrTokenFormatInvalid = New(40103, "invalid authorization format")
)

// 403xx — 权限错误
var (
	ErrForbidden              = New(40300, "forbidden")
	ErrInsufficientPermission = New(40301, "insufficient permissions")
)

// 404xx — 资源不存在（xx = 业务域）
var (
	ErrNotFound     = New(40400, "not found")
	ErrUserNotFound = New(40401, "user not found")
)

// 409xx — 冲突错误（xx = 业务域）
var (
	ErrConflict          = New(40900, "resource conflict")
	ErrUserAlreadyExists = New(40901, "username already exists")
)

// 500xx — 服务端错误
var (
	ErrInternal           = New(50000, "internal server error")
	ErrDatabaseError      = New(50001, "database error")
	ErrCacheError         = New(50002, "cache error")
	ErrServiceUnavailable = New(50300, "service unavailable")
)

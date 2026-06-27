// Package utils 提供统一的 API 响应格式
package utils

// APIResponse 统一 API 响应结构
// 与 Python 版 core/response.py 的 APIResponse 保持一致
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Msg     *string     `json:"msg"`
	Detail  interface{} `json:"detail,omitempty"`
}

// Success 返回成功响应 (code=200)
func Success(detail interface{}) APIResponse {
	msg := "ok"
	if detail == nil {
		detail = struct{}{}
	}
	return APIResponse{
		Code:    200,
		Message: "ok",
		Msg:     &msg,
		Detail:  detail,
	}
}

// Error 返回错误响应，message 会同时填充 message 和 msg 字段
func Error(code int, message string) APIResponse {
	msg := message
	return APIResponse{
		Code:    code,
		Message: message,
		Msg:     &msg,
	}
}

// ErrorDetail 返回带详情的错误响应
func ErrorDetail(code int, message string, detail interface{}) APIResponse {
	msg := message
	return APIResponse{
		Code:    code,
		Message: message,
		Msg:     &msg,
		Detail:  detail,
	}
}

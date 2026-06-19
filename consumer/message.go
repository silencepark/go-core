// Package consumer 提供消息消费者共享类型。
package consumer

import "context"

// MessageHandler 消息处理函数。key / value 为消息体的原始字节。
// 返回 error 表示处理失败，由 Runner 执行重试与 DLQ 投递。
type MessageHandler func(ctx context.Context, key, value []byte) error

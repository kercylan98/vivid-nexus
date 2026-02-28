package nexus

import "io"

// Session 表示底层连接抽象，由接入层（如 TCP、WebSocket）实现。
//
// 约定：具备 io.ReadWriteCloser 的读写与关闭能力；GetSessionId 返回该连接在 Nexus 内的唯一标识，
// 用于会话映射、Close(sessionId)、Send(sessionId, message) 等。同一 ID 的后续连接会替换旧会话。
type Session interface {
	io.ReadWriteCloser
	// GetSessionId 返回该会话在 Nexus 内的唯一 ID，用于关闭、发送等操作。
	GetSessionId() string
}

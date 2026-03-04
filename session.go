package nexus

import "io"

// Session 表示底层连接抽象，由接入层（如 TCP、WebSocket）实现。
type Session interface {
	io.ReadWriteCloser
	// GetSessionId 返回本会话的唯一标识。
	GetSessionId() string
}

// MetadataSession 在 Session 基础上提供接入层传入的元数据。
type MetadataSession interface {
	Session
	// Metadata 返回接入时附加的元数据，无则返回 nil。
	Metadata() map[string]any
}

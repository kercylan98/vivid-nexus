package nexus

import (
	"io"

	"github.com/kercylan98/vivid"
)

// Session 表示底层连接抽象，由接入层（如 TCP、WebSocket）实现。
//
// 约定：具备 io.ReadWriteCloser 的读写与关闭能力；GetSessionId 返回该连接在 Nexus 内的唯一标识，
// 用于会话映射、Close(sessionId)、Send(sessionId, message) 等。同一 ID 的后续连接会替换旧会话。
type Session interface {
	io.ReadWriteCloser
	// GetSessionId 返回该会话在 Nexus 内的唯一 ID，用于关闭、发送等操作。
	GetSessionId() string
}

// SessionContext 是业务在 OnConnected、OnDisconnected、OnMessage 中拿到的上下文。
//
// 除 vivid.ActorContext（Tell、Logger、Spawn 等）外，提供：
//   - GetSessionId：本会话唯一 ID；
//   - Close：关闭本会话（委托 Nexus 执行 Kill）；
//   - Send：向本会话写回数据（委托 Nexus 写回，并发安全）。
//
// 所有回调均在单一线程（sessionActor 邮箱）中串行执行，可安全使用 ctx。
type SessionContext interface {
	vivid.ActorContext
	GetSessionId() string
	Close()
	Send(message []byte) error
}

// SessionActor 由业务实现的会话逻辑接口，嵌入 vivid.Actor。
//
// 所有回调均在 sessionActor 的邮箱线程中串行执行，可安全使用 ctx 进行 Send、Close、Tell 等。
// message 在 OnMessage 中的生命周期仅在本次调用内有效，如需异步或长期持有须拷贝。
type SessionActor interface {
	vivid.Actor
	// OnConnected 在会话对应 Actor 启动后、读循环启动前调用，表示连接已就绪。
	OnConnected(ctx SessionContext)
	// OnDisconnected 在会话即将关闭时调用（Kill 处理中），之后底层 Session 会被 Close。
	OnDisconnected(ctx SessionContext)
	// OnMessage 在每收到一条由 SessionReader 读取并投递到邮箱的 []byte 时调用。
	OnMessage(ctx SessionContext, message []byte)
}

// SessionActorProvider 为每个新会话提供一个 SessionActor 实例。
//
// Nexus 在创建 sessionActor 时调用 Provide()；返回 nil 或 error 则会话不启动。
// 可用于按会话创建独立业务实例，或返回单例（由实现方保证线程安全）。
type SessionActorProvider interface {
	Provide() (SessionActor, error)
}

// SessionReader 会话数据读取器接口，由框架在独立 goroutine 中循环调用 Read。
//
// 调度约定：Read() → 处理返回数据 → 再次 Read()，同一处理周期内不并行读取。
// 每次 Read() 返回的 data 仅保证在本处理周期内有效，下一次 Read() 可能复用同一缓冲区。
//
// 返回的 []byte 所有权与生命周期（调用方必须遵守）：
//   - 所有权：调用方不拥有底层存储；data 是实现方内部缓冲区的视图，下次 Read 或关闭时可能被复用/覆盖。
//   - 生命周期：data 仅在本次 Read 返回到同一 Reader 下一次 Read 被调用前有效；跨周期或异步使用须自行拷贝（如 copy、bytes.Clone）。
//   - 实现方应线程安全，可复用内部 buffer 以支持零拷贝。
//
// 返回值约定：n 为读到的字节数且 0 <= n <= len(data)；data 为 nil 当且仅当 n == 0；
// 遇 EOF 时应先返回已读数据（n > 0, err == nil），下次 Read 再返回 (0, nil, io.EOF)。
type SessionReader interface {
	Read() (n int, data []byte, err error)
}

// SessionReaderProvider 为指定 Session 提供对应的 SessionReader 实例。
//
// 要求实现线程安全；Provide 在 sessionActor 的 Prelaunch 阶段调用。
// 返回的 SessionReader 不可为 nil（框架会校验并返回错误）。
type SessionReaderProvider interface {
	Provide(session Session) (SessionReader, error)
}

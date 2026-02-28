package nexus

import (
	"io"
	"sync"
)

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

// SessionReaderFN 是 SessionReader 的函数式适配器类型。
//
// 适用于无状态或简单读取逻辑，可直接用函数实现读取而无需定义结构体。
// 注意：SessionReader 接口的 Read 为无参，框架通过 SessionReaderProvider.Provide(session) 得到已绑定 session 的 Reader；
// 使用本适配器时需在 Provider 中返回一个包装了 session 与 SessionReaderFN 的 Reader（如默认实现的 newDefaultSessionReader 即按 Session 绑定）。
// 返回的 data 所有权与生命周期同 SessionReader 约定：仅在本轮返回到下次 Read 前有效，跨周期使用须拷贝。
type SessionReaderFN func(session Session) (n int, data []byte, err error)

// Read 调用函数并返回读取结果；[]byte 所有权与生命周期与 SessionReader 约定一致。
func (fn SessionReaderFN) Read(session Session) (n int, data []byte, err error) {
	return fn(session)
}

// SessionReaderProviderFN 是 SessionReaderProvider 的函数式适配器类型。
//
// 便于用函数根据 Session 返回对应 SessionReader，无需定义新类型。
// 示例：nexus.SessionReaderProviderFN(nexus.newDefaultSessionReader) 或自定义 func(session Session) (SessionReader, error)。
// 要求实现线程安全；返回的 SessionReader 不可为 nil。
type SessionReaderProviderFN func(session Session) (SessionReader, error)

// Provide 调用函数并返回该 Session 对应的 SessionReader。
func (fn SessionReaderProviderFN) Provide(session Session) (SessionReader, error) {
	return fn(session)
}

// newDefaultSessionReader 返回基于给定 Session 的默认 SessionReader，适用于按字节流读取的简单场景。
// 可与 SessionReaderProviderFN 配合：Provide 中 return NewDefaultSessionReader(session), nil。
func newDefaultSessionReader(session Session) (SessionReader, error) {
	return &defaultSessionReader{session: session}, nil
}

// defaultSessionReader 基于 Session 的默认 SessionReader 实现：
// 复用内部缓冲区、线程安全、正确处理带数据的 EOF。
type defaultSessionReader struct {
	session    Session
	mu         sync.Mutex
	buf        []byte // 复用缓冲区；Read 返回的 data 为 buf 的切片，仅在下一次 Read 前有效
	pendingErr error  // 与最后一次读同批的 EOF，下次 Read 时返回
}

// Read 从 Session 读入内部缓冲区并返回本批数据的长度与切片。
//
// 返回的 []byte 所有权与生命周期（与 SessionReader 接口约定一致）：
//   - 所有权：data 指向 defaultSessionReader 的内部 buf，调用方不拥有底层存储；实现方在下次 Read 时可能覆盖 buf。
//   - 生命周期：data 仅在本次 Read 返回后、到同一 *defaultSessionReader 上下一次 Read 被调用前有效；此后不得再访问 data。
//   - 若框架或业务需要在本次处理周期之外使用数据，必须在此之前完成 copy 或 bytes.Clone，不得将 data 传入异步逻辑或长期持有。
//
// 线程安全：单次 Read 在 mu 下执行，与 SessionReader 的“同一周期内不并行 Read”的用法兼容。
func (r *defaultSessionReader) Read() (n int, data []byte, err error) {
	const defaultReadBufferSize = 4096

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session == nil {
		return 0, nil, io.ErrClosedPipe
	}

	if r.pendingErr != nil {
		err = r.pendingErr
		r.pendingErr = nil
		return 0, nil, err
	}

	if cap(r.buf) < defaultReadBufferSize {
		r.buf = make([]byte, defaultReadBufferSize)
	}

	n, err = r.session.Read(r.buf)
	if n > 0 {
		data = r.buf[:n:n]
	}
	if err != nil && err != io.EOF {
		return n, data, err
	}
	if err == io.EOF {
		r.pendingErr = io.EOF
		return n, data, nil
	}
	return n, data, nil
}

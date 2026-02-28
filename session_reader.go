package nexus

import (
	"io"
	"sync"
)

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

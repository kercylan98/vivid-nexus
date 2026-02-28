// Package nexus 提供基于 vivid Actor 的会话管理层。
// 本文件定义其对外核心接口 Nexus。
package nexus

import "github.com/kercylan98/vivid"

// Nexus 是会话托管与消息分发的统一入口。
//
// 实现该接口的类型（如通过 New 返回的 Actor）负责：
//   - 接管底层连接（Session），为每个连接创建并管理独立的 sessionActor；
//   - 按 sessionId 关闭会话、单发/群发/广播消息；
//   - 与 vivid 的 Actor 生命周期集成（Kill 时清理所有托管会话）。
//
// 典型用法：将实现了 Session 的连接通过 TakeoverSession 交给 Nexus，
// 业务通过 SessionActorProvider 提供每会话的回调逻辑（OnConnected/OnDisconnected/OnMessage）。
type Nexus interface {
	// Actor 返回本 Nexus 对应的 vivid.Actor 实例。
	// 用于将 Nexus 注册到 vivid 的 Actor 系统，或通过 ref 向 Nexus 投递消息（如 Session）。
	Actor() vivid.Actor

	// TakeoverSession 接管一个会话，由 Nexus 为其创建 sessionActor 并启动读循环。
	// 若该 Session 的 GetSessionId() 已存在托管会话，则先关闭并移除旧会话，再接管新会话。
	TakeoverSession(session Session)

	// Close 关闭指定 sessionId 的托管会话。
	// 若存在则 Kill 对应 sessionActor 并移除映射；若不存在则无操作。可安全重复调用，并发安全。
	Close(sessionId string)

	// Send 向指定 sessionId 的会话发送一条消息（写入底层 Session）。
	// 若 message 为空则返回 nil；若会话不存在或已关闭则返回 nil（不报错）。同一会话的写由内部锁串行化，并发安全。
	Send(sessionId string, message []byte) error

	// SendTo 向 sessionIds 中的每个会话发送 message，重复的 sessionId 只发送一次。
	// 若 sessionIds 或 message 为空则直接返回。可选 errorHandler：任一会话发送失败时调用，返回 true 则中止后续发送。
	SendTo(sessionIds []string, message []byte, errorHandler ...SendErrorHandler)

	// Broadcast 向当前所有托管会话广播 message。
	// 可选 errorHandler：任一会话发送失败时调用，返回 true 则中止后续发送。
	Broadcast(message []byte, errorHandler ...SendErrorHandler)
}

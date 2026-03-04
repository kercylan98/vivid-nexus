// Package nexus 提供基于 vivid 的会话管理层。
package nexus

import "github.com/kercylan98/vivid"

// Nexus 是会话托管与消息分发的入口。
type Nexus interface {
	// Inject 在特定 ActorSystem 中注入并创建 Nexus Actor，而后返回对应的 ActorRef。
	//
	// 该函数在多次调用时会始终返回相同的 ActorRef，不会多次创建 Nexus Actor。即便是不同的 ActorSystem。
	Inject(system vivid.ActorSystem, options ...vivid.ActorOption) (vivid.ActorRef, error)

	// TakeoverSession 接管会话并开始管理其生命周期与读写。
	TakeoverSession(session Session)

	// Close 关闭指定 sessionId 的会话，不存在则无操作。
	Close(sessionId string)

	// Send 向指定 sessionId 的会话发送消息，会话不存在或已关闭则返回 nil。
	Send(sessionId string, message []byte) error

	// SendTo 向 sessionIds 中的每个会话发送 message，重复 id 只发一次。
	// errorHandler 在任一会话发送失败时调用，返回 true 则中止后续发送。
	SendTo(sessionIds []string, message []byte, errorHandler ...SendErrorHandler)

	// Broadcast 向当前所有托管会话广播 message。
	// errorHandler 在任一会话发送失败时调用，返回 true 则中止后续发送。
	Broadcast(message []byte, errorHandler ...SendErrorHandler)
}

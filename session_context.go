package nexus

import "github.com/kercylan98/vivid"

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

// sessionContext 将 sessionInfo 与 ActorContext 组合为 SessionContext，供 sessionActor 注入后传给业务。
type sessionContext struct {
	*sessionInfo
	vivid.ActorContext
}

func (c *sessionContext) Close() {
	c.sessionInfo.operator.Close(c.GetSessionId())
}

func (c *sessionContext) Send(message []byte) error {
	return c.sessionInfo.operator.Send(c.GetSessionId(), message)
}

func (c *sessionContext) GetSessionId() string {
	return c.Session.GetSessionId()
}

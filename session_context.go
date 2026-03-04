package nexus

import "github.com/kercylan98/vivid"

// SessionContext 在 OnConnected、OnMessage、OnDisconnected 中提供当前会话的上下文。
//
// 内嵌 vivid.ActorContext，并扩展本会话的 ID、关闭、发送及接入层传入的元数据访问。
type SessionContext interface {
	vivid.ActorContext
	// GetSessionId 返回本会话的唯一标识。
	GetSessionId() string
	// Close 关闭本会话。
	Close()
	// Send 向本会话发送数据，会话已关闭时返回 error。
	Send(message []byte) error
	// GetMetadata 返回 key 对应的元数据值，不存在返回 nil。
	GetMetadata(key string) any
	// GetMetadataWithDefault 返回 key 对应的元数据值，不存在返回 defaultValue。
	GetMetadataWithDefault(key string, defaultValue any) any
	// GetMetadataWithExists 返回 key 对应的元数据值及是否存在。
	GetMetadataWithExists(key string) (any, bool)
	// HasMetadata 报告 key 是否存在于元数据中。
	HasMetadata(key string) bool
}

// sessionContext 实现 SessionContext。
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

func (c *sessionContext) GetMetadata(key string) any {
	return c.sessionInfo.metadata[key]
}

func (c *sessionContext) GetMetadataWithDefault(key string, defaultValue any) any {
	if val, ok := c.sessionInfo.metadata[key]; ok {
		return val
	}
	return defaultValue
}

func (c *sessionContext) GetMetadataWithExists(key string) (any, bool) {
	val, ok := c.sessionInfo.metadata[key]
	return val, ok
}

func (c *sessionContext) HasMetadata(key string) bool {
	return c.sessionInfo.metadata[key] != nil
}

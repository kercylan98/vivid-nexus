package nexus

import "github.com/kercylan98/vivid"

// SendErrorHandler 在 Broadcast/SendTo 中某会话发送失败时被调用。
//
// 参数：sessionId 为当前发送的会话 ID；sessionContext 在 Broadcast 中目前传 nil；err 为本次 Write 的错误。
// 返回值：abort 为 true 时停止向后续会话发送，为 false 时继续。
type SendErrorHandler = func(sessionId string, sessionContext SessionContext, err error) (abort bool)

type operator struct {
	*actor
	vivid.ActorContext
}

// TakeoverSession 用于接管一个已存在的 Session，并存入 operator 的会话管理中。
// 如果 sessionId 已存在，原有会话会被关闭并替换为新会话。
//
// 也可以直接通过将 Session Tell 到 Nexus Actor 的 ref 来达到等效的作用。
func (o *operator) TakeoverSession(session Session) {
	o.operator.actor.TellSelf(session)
}

// Close 关闭指定 ID 的会话。
//
// 若该 sessionId 存在托管会话，则从映射中移除并 Kill 对应 sessionActor（底层 Session 由 session 侧关闭）；
// 若不存在则无操作，可安全重复调用。并发安全。
func (o *operator) Close(sessionId string) {
	o.sessionLock.Lock()
	defer o.sessionLock.Unlock()

	if session, ok := o.sessions[sessionId]; ok {
		delete(o.sessions, sessionId)
		o.ActorContext.Kill(session.ref, false, "close session")
	}
}

// Send 向指定 ID 的会话推送消息（写回底层 Session）。
//
// 若 message 为空则直接返回 nil；若 sessionId 不存在或已关闭则返回 nil（不返回错误）。
// 同一会话的多次 Send 由 session 侧 writeLock 串行化，并发安全。
func (o *operator) Send(sessionId string, message []byte) error {
	if len(message) == 0 {
		return nil
	}

	o.sessionLock.RLock()
	defer o.sessionLock.RUnlock()

	if info, ok := o.sessions[sessionId]; ok {
		info.writeLock.Lock()
		defer info.writeLock.Unlock()
		_, err := info.Session.Write(message)
		return err
	}
	return nil
}

// SendTo 向 sessionIds 中的每个会话推送 message，对重复的 sessionId 只发送一次。
//
// 若 sessionIds 或 message 为空则直接返回。若提供了 errorHandler，则任一会话发送失败时调用
// handler(sessionId, nil, err)；若某次 handler 返回 true 则中止后续发送。
func (o *operator) SendTo(sessionIds []string, message []byte, errorHandler ...SendErrorHandler) {
	if len(sessionIds) == 0 || len(message) == 0 {
		return
	}

	var err error
	var sended = make(map[string]struct{})
	for _, sessionId := range sessionIds {
		if _, ok := sended[sessionId]; ok {
			continue
		}
		sended[sessionId] = struct{}{}
		err = o.Send(sessionId, message)
		if err != nil && len(errorHandler) > 0 {
			for _, handler := range errorHandler {
				if abort := handler(sessionId, nil, err); abort {
					return
				}
			}
		}
	}
}

// Broadcast 向当前所有托管会话推送 message。
//
// 先复制当前 sessions 的 key 列表再逐条 Send，避免持锁过久。若提供 errorHandler，
// 则任一会话发送失败时调用 handler；若某次 handler 返回 true 则中止后续发送。
func (o *operator) Broadcast(message []byte, errorHandler ...SendErrorHandler) {
	var sessionIds = make([]string, 0, len(o.sessions))
	o.sessionLock.RLock()
	for sessionId := range o.sessions {
		sessionIds = append(sessionIds, sessionId)
	}
	o.sessionLock.RUnlock()
	o.SendTo(sessionIds, message, errorHandler...)
}

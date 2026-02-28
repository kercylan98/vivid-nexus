// Package nexus 基于 vivid Actor 模型提供会话管理层：将每个连接抽象为 Session，
// 由 Nexus 统一托管并 Kill，每个 Session 对应一个 sessionActor，业务通过 SessionActor 实现连接/断开/消息回调。
package nexus

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor = (*actor)(nil)
)

// New 构造 Nexus 实例，用于集中托管会话（每个 Session 对应一个 sessionActor）。
//
// 参数：
//   - provider：为每个新会话提供业务侧 SessionActor，不可为 nil，否则返回错误。
//   - options：可选配置，如 WithSessionReaderProvider；未传时使用 NewOptions() 的默认值。
//
// 返回的 *actor 同时实现 vivid.Actor（需注册到 vivid 并接收 Session 以托管新连接）
// 以及 operator 能力：Close(sessionId)、Send(sessionId, message)、Broadcast(message)。
// 使用方式：将返回值注册到 vivid，向该 Actor 发送 Session 即可托管新连接；持有该指针可调用 Close/Send/Broadcast。
func New(provider SessionActorProvider, options ...Option) (*actor, error) {
	if provider == nil {
		return nil, errors.New("session actor provider is nil")
	}

	opts := NewOptions(options...)
	a := &actor{
		options:  *opts,
		provider: provider,
	}
	a.operator = &operator{
		actor: a,
	}

	return a, nil
}

// actor 集中管理所有托管会话：收到 Session 时为其创建 sessionActor，
// 收到 OnKilled 时从 sessions 中移除对应 ref；OnKill 时重置并 Kill 所有子会话。
type actor struct {
	*operator
	options     Options
	provider    SessionActorProvider
	sessions    map[string]*sessionInfo // sessionId -> sessionInfo，用于替换同 id 会话与清理
	sessionLock sync.RWMutex            // 用于保护 sessions 的读写操作
}

// OnReceive 实现 vivid.Actor：分发 OnLaunch、Session、OnKilled、OnKill，其它类型打 Warn 日志。
func (n *actor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		n.onLaunch(ctx)
	case Session:
		n.onSession(ctx, msg)
	case *vivid.OnKilled:
		n.onKilled(ctx, msg)
	case *vivid.OnKill:
		n.onKill(ctx)
	default:
		ctx.Logger().Warn("NexusActor received unsupported message type", log.String("expected", fmt.Sprintf("%T", (*Session)(nil))), log.String("received", fmt.Sprintf("%T", msg)))
	}
}

// onLaunch 在 Actor 启动时初始化 sessions 映射。
func (n *actor) onLaunch(ctx vivid.ActorContext) {
	n.operator.ActorContext = ctx
	n.reset(ctx)
}

// onKill 在 Actor 被关闭时清理所有托管会话并重置 map。
func (n *actor) onKill(ctx vivid.ActorContext) {
	n.reset(ctx)
}

// reset 若 sessions 非 nil 则对所有已托管 ref 执行 Kill，然后重建空 map；否则仅初始化 map。
func (n *actor) reset(ctx vivid.ActorContext) {
	n.sessionLock.Lock()
	defer n.sessionLock.Unlock()

	if n.sessions == nil {
		n.sessions = make(map[string]*sessionInfo)
		return
	}
	for id, sessionRef := range n.sessions {
		delete(n.sessions, id)
		ctx.Kill(sessionRef.ref, false, "cleanup session")
	}
	n.sessions = make(map[string]*sessionInfo)
}

// onKilled 处理子 session actor 终止：仅当 map 中该 id 仍指向该 ref 时删除，
// 避免同一 id 已替换为新 ref 时误删新会话，保证严格一致性。
func (n *actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
	if msg.Ref.Equals(ctx.Ref()) {
		// 如果被杀死的引用是自己，则直接返回
		return
	}

	killedRef := msg.Ref

	n.sessionLock.Lock()
	defer n.sessionLock.Unlock()

	for id, info := range n.sessions {
		if info != nil && info.ref.Equals(killedRef) {
			delete(n.sessions, id)
			break
		}
	}
}

// onSession 为传入的 Session 创建 sessionActor 并托管：若同 id 已存在则先 Kill 旧 ref 再写入新 ref。
func (n *actor) onSession(ctx vivid.ActorContext, session Session) {
	id := session.GetSessionId()
	sessionInfo := newSessionInfo(n.operator, session)
	sessionActor := newSessionActor(sessionInfo, n.provider, n.options)
	ref, err := ctx.ActorOf(sessionActor, vivid.WithActorName(id))
	if err != nil {
		ctx.Logger().Error("session actor spawn failed", log.String("id", id), log.Any("err", err))
		if err = session.Close(); err != nil {
			ctx.Logger().Error("session close failed", log.String("id", id), log.Any("err", err))
		}
		return
	}

	sessionActor.context.sessionInfo.ref = ref

	n.sessionLock.Lock()
	defer n.sessionLock.Unlock()

	if existing, ok := n.sessions[id]; ok {
		ctx.Kill(existing.ref, false, "close existing session")
	}

	n.sessions[id] = sessionInfo
}

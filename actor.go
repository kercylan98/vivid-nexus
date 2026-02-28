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
	_ vivid.Actor = (*Actor)(nil)
	_ Nexus       = (*Actor)(nil)
)

// New 构造 Nexus 实例，用于集中托管会话
//
// 参数：
//   - provider：为每个新会话提供业务侧 SessionActor，不可为 nil，否则返回错误。
//   - options：可选配置，如 WithSessionReaderProvider；未传时使用 NewOptions() 的默认值。
func New(provider SessionActorProvider, options ...Option) (Nexus, error) {
	if provider == nil {
		return nil, errors.New("session actor provider is nil")
	}

	opts := NewOptions(options...)
	a := &Actor{
		options:  *opts,
		provider: provider,
	}
	a.operator = &operator{
		actor: a,
	}

	return a, nil
}

// Actor 集中管理所有托管会话：收到 Session 时为其创建 sessionActor，
// 收到 OnKilled 时从 sessions 中移除对应 ref；OnKill 时重置并 Kill 所有子会话。
type Actor struct {
	*operator
	options     Options
	provider    SessionActorProvider
	sessions    map[string]*sessionInfo // sessionId -> sessionInfo，用于替换同 id 会话与清理
	sessionLock sync.RWMutex            // 用于保护 sessions 的读写操作
}

func (n *Actor) Actor() vivid.Actor {
	return n
}

func (n *Actor) OnReceive(ctx vivid.ActorContext) {
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

func (n *Actor) onLaunch(ctx vivid.ActorContext) {
	n.operator.actorContext = ctx
	n.reset(ctx)
}

func (n *Actor) onKill(ctx vivid.ActorContext) {
	n.reset(ctx)
}

func (n *Actor) reset(ctx vivid.ActorContext) {
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

func (n *Actor) onKilled(ctx vivid.ActorContext, msg *vivid.OnKilled) {
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
			ctx.Logger().Debug("session closed", log.String("session_id", id), log.Int("online_count", len(n.sessions)))
			break
		}
	}
}

func (n *Actor) onSession(ctx vivid.ActorContext, session Session) {
	id := session.GetSessionId()

	// 先行加锁，避免 OnLaunch 先执行后，还未注册到 sessions 中就推送消息
	n.sessionLock.Lock()
	defer n.sessionLock.Unlock()

	sessionInfo := newSessionInfo(n.operator, session)
	sessionActor := newSessionActor(sessionInfo, n.provider, n.options)
	ref, err := ctx.ActorOf(sessionActor)
	if err != nil {
		ctx.Logger().Error("session actor spawn failed", log.String("id", id), log.Any("err", err))
		if err = session.Close(); err != nil {
			ctx.Logger().Error("session close failed", log.String("id", id), log.Any("err", err))
		}
		return
	}

	sessionActor.context.sessionInfo.ref = ref

	if existing, ok := n.sessions[id]; ok {
		ctx.Logger().Debug("close existing session", log.String("session_id", id))
		ctx.Kill(existing.ref, false, "close existing session")
	}

	n.sessions[id] = sessionInfo

	ctx.Logger().Debug("session opened", log.String("session_id", id), log.Int("online_count", len(n.sessions)))
}

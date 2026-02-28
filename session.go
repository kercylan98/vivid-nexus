package nexus

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/kercylan98/vivid"
	"github.com/kercylan98/vivid/pkg/log"
)

var (
	_ vivid.Actor          = (*sessionActor)(nil)
	_ vivid.PrelaunchActor = (*sessionActor)(nil)
)

func newSessionInfo(operator *operator, session Session) *sessionInfo {
	return &sessionInfo{
		operator: operator,
		Session:  session,
	}
}

type sessionInfo struct {
	*operator
	Session
	ref       vivid.ActorRef
	writeLock sync.Mutex
}

// SessionActorProviderFN 是 SessionActorProvider 的函数式适配器类型。
//
// 便于用匿名函数或闭包实现 SessionActorProvider，无需定义新结构体。
// 示例：nexus.SessionActorProviderFN(func() (nexus.SessionActor, error) { return myActor, nil })。
type SessionActorProviderFN func() (SessionActor, error)

// Provide 调用函数并返回 SessionActor；若函数返回 nil 或 error，Nexus 不会启动该会话。
func (fn SessionActorProviderFN) Provide() (SessionActor, error) {
	return fn()
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

// newSessionActor 构造与给定 sessionInfo 绑定的 sessionActor，Prelaunch 前不会启动读循环。
func newSessionActor(sessionInfo *sessionInfo, provider SessionActorProvider, options Options) *sessionActor {
	return &sessionActor{
		context:  &sessionContext{sessionInfo: sessionInfo},
		options:  options,
		provider: provider,
		messageC: make(chan struct{}),
	}
}

// sessionActor 将单个 Session 封装为 vivid Actor，负责 Prelaunch/Launch/Kill 与独立读循环，
// 通过 messageC 与业务处理做背压：读一条、处理完、再读下一条。
type sessionActor struct {
	context              *sessionContext // 组合 Session + ActorContext，传给业务
	options              Options         // 含 SessionReaderProvider 等配置
	provider             SessionActorProvider
	reader               SessionReader // 由 SessionReaderProvider 按 Session 提供
	externalSessionActor SessionActor  // 业务实现的回调对象
	closed               atomic.Bool   // 仅 CAS/Load，保证 readLoop 与 onKill 间可见性
	messageC             chan struct{} // 背压：onMessage 处理完后发送，readLoop 接收后继续读
}

// OnPrelaunch 在 Actor 真正启动前执行：拉取 SessionActor 与 SessionReader，任一失败则会话不启动。
func (a *sessionActor) OnPrelaunch(ctx vivid.PrelaunchContext) (err error) {
	if a.closed.Load() {
		return errors.New("session already closed")
	}

	externalSessionActor, err := a.provider.Provide()
	if err != nil {
		return err
	}
	if externalSessionActor == nil {
		return errors.New("session actor provider provide nil session actor")
	}
	a.externalSessionActor = externalSessionActor

	a.reader, err = a.options.SessionReaderProvider.Provide(a.context.Session)
	if err != nil {
		return err
	}
	if a.reader == nil {
		return errors.New("session reader provider provide nil session reader")
	}
	return err
}

// OnReceive 分发邮箱消息：OnLaunch 触发连接回调并启动读循环，OnKill 做幂等关闭，[]byte 交给 OnMessage。
func (a *sessionActor) OnReceive(ctx vivid.ActorContext) {
	switch msg := ctx.Message().(type) {
	case *vivid.OnLaunch:
		a.onLaunch(ctx)
	case *vivid.OnKill:
		a.onKill(ctx, msg)
	case []byte:
		a.onMessage(ctx, msg)
	}
}

// onLaunch 在 Actor 启动时调用：先触发 OnConnected，再在独立 goroutine 中启动 readLoop，避免读阻塞邮箱。
func (a *sessionActor) onLaunch(ctx vivid.ActorContext) {
	// 注入 context
	a.context.ActorContext = ctx

	defer func() {
		// 如果在 OnConnected 或 readLoop 中发生 panic，则杀死自己，避免异常连接进入
		if err := recover(); err != nil {
			ctx.Logger().Error("session actor onLaunch panic", log.String("id", a.context.GetSessionId()), log.Any("err", err))
			ctx.Kill(ctx.Ref(), false, "session actor onLaunch panic")
		}
	}()

	a.externalSessionActor.OnConnected(a.context)
	go a.readLoop(ctx)
}

// onKill 幂等关闭会话：仅首次 CAS 成功时执行 defer（close messageC、Close Session、OnDisconnected）。
func (a *sessionActor) onKill(ctx vivid.ActorContext, msg *vivid.OnKill) {
	if !a.closed.CompareAndSwap(false, true) {
		return
	}
	defer func() {
		close(a.messageC)
		a.context.sessionInfo.writeLock.Lock()
		defer a.context.sessionInfo.writeLock.Unlock()
		if err := a.context.Session.Close(); err != nil {
			ctx.Logger().Error("session close failed", log.String("id", a.context.GetSessionId()), log.Any("reason", msg), log.Any("err", err))
		}
	}()

	a.externalSessionActor.OnDisconnected(a.context)
}

// readLoop 在独立 goroutine 中循环读取；每次读到的数据 TellSelf 后通过 <-messageC 等待处理完成再读下一条。
// 严禁在此 goroutine 内使用 ctx 做 ActorSpawn 等并发非安全操作；异常或 EOF 时 defer 会 Kill 本 Actor。
func (a *sessionActor) readLoop(ctx vivid.ActorContext) {
	var err error
	var n int
	var data []byte

	defer func() {
		var reason = "session read loop closed"
		if err := recover(); err != nil {
			reason = "session read loop panic"
			ctx.Logger().Error(reason, log.Any("err", err))
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				reason = "session read EOF"
				ctx.Logger().Debug(reason, log.String("id", a.context.GetSessionId()))
			} else {
				reason = "session read failed, err: " + err.Error()
				ctx.Logger().Error(reason, log.String("id", a.context.GetSessionId()))
			}
		}

		ctx.Kill(ctx.Ref(), false, reason)
	}()

	for !a.closed.Load() {
		n, data, err = a.reader.Read()
		if err != nil {
			return
		}
		if dataLen := len(data); n != dataLen {
			return
		}
		ctx.TellSelf(data)
		<-a.messageC
	}
}

// onMessage 处理邮箱中的 []byte：业务处理完成后若未关闭则向 messageC 发送信号，以解除 readLoop 的背压等待。
func (a *sessionActor) onMessage(_ vivid.ActorContext, message []byte) {
	defer func() {
		if !a.closed.Load() {
			a.messageC <- struct{}{}
		}
	}()
	a.externalSessionActor.OnMessage(a.context, message)
}

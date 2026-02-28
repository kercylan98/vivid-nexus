package session

import (
	nexus "github.com/kercylan98/vivid-nexus"
)

var (
	_ nexus.SessionActor = (*Actor)(nil)
)

type Actor struct {
}

func (a *Actor) OnConnected(ctx nexus.SessionContext) {
	ctx.Send([]byte("connected:" + ctx.GetSessionId()))
	ctx.Send([]byte("commands: close"))
	ctx.Send([]byte("  - close: this command will close the session"))
	ctx.Send([]byte("other commands: echo"))
}

func (a *Actor) OnDisconnected(ctx nexus.SessionContext) {
	ctx.Send([]byte("disconnected:" + ctx.GetSessionId()))
}

func (a *Actor) OnMessage(ctx nexus.SessionContext, message []byte) {
	echo := make([]byte, len(message))
	switch string(message) {
	case "close":
		ctx.Close()
	default:
		copy(echo, message)
		ctx.Send(echo)
	}

}

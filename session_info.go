package nexus

import (
	"sync"

	"github.com/kercylan98/vivid"
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

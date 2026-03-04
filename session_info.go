package nexus

import (
	"maps"
	"sync"

	"github.com/kercylan98/vivid"
)

func newSessionInfo(operator *operator, session Session) *sessionInfo {
	info := &sessionInfo{
		operator: operator,
		Session:  session,
	}
	if metadataSession, ok := session.(MetadataSession); ok {
		info.metadata = maps.Clone(metadataSession.Metadata())
	}
	return info
}

type sessionInfo struct {
	*operator
	Session
	ref       vivid.ActorRef // Session 自身对应 ActorRef
	writeLock sync.Mutex     // 写锁，用于保证写操作的顺序性
	metadata  map[string]any // 元数据，用于在回调间携带业务状态
}

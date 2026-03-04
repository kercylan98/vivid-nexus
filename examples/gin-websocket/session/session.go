package session

import (
	"io"
	"sync/atomic"

	"github.com/gorilla/websocket"
	nexus "github.com/kercylan98/vivid-nexus"
)

var (
	_ nexus.Session         = (*Session)(nil)
	_ nexus.MetadataSession = (*Session)(nil)
)

func NewSession(sessionId string, conn *websocket.Conn, metadata map[string]any) *Session {
	return &Session{
		sessionId: sessionId,
		conn:      conn,
		metadata:  metadata,
	}
}

type Session struct {
	sessionId string
	conn      *websocket.Conn
	closed    atomic.Bool
	metadata  map[string]any
}

func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return s.conn.Close()
}

func (s *Session) GetSessionId() string {
	return s.sessionId
}

func (s *Session) Read(p []byte) (n int, err error) {
	_, message, err := s.conn.ReadMessage()
	if err != nil {
		if s.closed.Load() {
			return 0, io.EOF
		}
		return 0, err
	}

	n = copy(p, message)
	return n, nil
}

func (s *Session) Write(p []byte) (n int, err error) {
	return len(p), s.conn.WriteMessage(websocket.TextMessage, p)
}

func (s *Session) Metadata() map[string]any {
	return s.metadata
}

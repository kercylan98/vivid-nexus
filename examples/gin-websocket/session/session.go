package session

import (
	"io"
	"sync/atomic"

	"github.com/gorilla/websocket"
	nexus "github.com/kercylan98/vivid-nexus"
)

var (
	_ nexus.Session = (*Session)(nil)
)

func NewSession(sessionId string, conn *websocket.Conn) *Session {
	return &Session{
		sessionId: sessionId,
		conn:      conn,
	}
}

type Session struct {
	sessionId string
	conn      *websocket.Conn
	closed    atomic.Bool
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

package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingInterval = 25 * time.Second
	pongTimeout  = 60 * time.Second
)

type connState struct {
	conn      *websocket.Conn
	lastPong  atomic.Int64
	send      chan outbound
	done      chan struct{}
	closeOnce sync.Once
}

type ConnectionManager struct {
	mu       sync.Mutex
	conns    map[*websocket.Conn]*connState
	listener net.Listener
	server   *http.Server
	stopOnce sync.Once
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conns: make(map[*websocket.Conn]*connState),
	}
}

func (m *ConnectionManager) Run(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.handleWS(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	m.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.listener = ln

	return m.server.Serve(ln)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (m *ConnectionManager) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	state := &connState{
		conn: conn,
		send: make(chan outbound, 16),
		done: make(chan struct{}),
	}
	state.lastPong.Store(time.Now().UnixNano())
	conn.SetPongHandler(func(string) error {
		state.lastPong.Store(time.Now().UnixNano())
		return nil
	})

	m.addConn(state)

	go m.readLoop(state)
	go m.writeLoop(state)
}

func (m *ConnectionManager) readLoop(state *connState) {
	conn := state.conn
	defer func() {
		m.removeConn(conn)
		state.close()
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if string(msg) == "stop-listening" {
			m.stopListening()
			continue
		}

		select {
		case state.send <- outbound{mt: mt, data: msg}:
		case <-state.done:
			return
		}
	}
}

type outbound struct {
	mt   int
	data []byte
}

func (m *ConnectionManager) writeLoop(state *connState) {
	conn := state.conn
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-state.send:
			if err := writeMessage(conn, msg.mt, msg.data); err != nil {
				state.close()
				return
			}
		case <-ticker.C:
			now := time.Now()
			last := time.Unix(0, state.lastPong.Load())
			if now.Sub(last) > pongTimeout {
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "pong timeout"),
					now.Add(2*time.Second),
				)
				state.close()
				return
			}
			if err := conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				now.Add(2*time.Second),
			); err != nil {
				state.close()
				return
			}
		case <-state.done:
			return
		}
	}
}

func (m *ConnectionManager) addConn(state *connState) {
	m.mu.Lock()
	m.conns[state.conn] = state
	m.mu.Unlock()
}

func (m *ConnectionManager) removeConn(conn *websocket.Conn) {
	m.mu.Lock()
	delete(m.conns, conn)
	m.mu.Unlock()
}

func writeMessage(conn *websocket.Conn, mt int, data []byte) error {
	if mt == websocket.PingMessage || mt == websocket.PongMessage || mt == websocket.CloseMessage {
		return conn.WriteControl(mt, data, time.Now().Add(2*time.Second))
	}
	return conn.WriteMessage(mt, data)
}

func (c *connState) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func (m *ConnectionManager) stopListening() {
	m.stopOnce.Do(func() {
		if m.listener != nil {
			_ = m.listener.Close()
		}
	})
}

func main() {
	manager := NewConnectionManager()
	if err := manager.Run(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Allow in-flight handlers to exit gracefully.
	_ = manager.server.Shutdown(context.Background())
}

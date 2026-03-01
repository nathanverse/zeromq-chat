package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pebbe/zmq4"
)

const (
	pingInterval = 25 * time.Second
	pongTimeout  = 60 * time.Second
	lobbyRoom    = "lobby"
)

type connState struct {
	conn      *websocket.Conn
	roomID    string
	lastPong  atomic.Int64
	send      chan outbound
	done      chan struct{}
	closeOnce sync.Once
}

type roomMessage struct {
	roomID string
	mt     int
	data   []byte
}

type pubRequest struct {
	roomID string
	mt     int
	data   []byte
}

type subRequest struct {
	roomID string
	add    bool
}

type BrokerClient struct {
	ctx      *zmq4.Context
	pubAddr  string
	subAddr  string
	pubCh    chan pubRequest
	subCmdCh chan subRequest
	recvCh   chan roomMessage
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewBrokerClient(pubAddr, subAddr string) (*BrokerClient, error) {
	ctx, err := zmq4.NewContext()
	if err != nil {
		return nil, err
	}

	b := &BrokerClient{
		ctx:      ctx,
		pubAddr:  pubAddr,
		subAddr:  subAddr,
		pubCh:    make(chan pubRequest, 512),
		subCmdCh: make(chan subRequest, 512),
		recvCh:   make(chan roomMessage, 512),
		done:     make(chan struct{}),
	}

	b.wg.Add(2)
	go b.pubLoop()
	go b.subLoop()
	return b, nil
}

func (b *BrokerClient) Publish(roomID string, mt int, data []byte) {
	msg := pubRequest{roomID: roomID, mt: mt, data: append([]byte(nil), data...)}
	select {
	case b.pubCh <- msg:
	case <-b.done:
	default:
		log.Printf("broker publish dropped room=%s reason=queue_full", roomID)
	}
}

func (b *BrokerClient) Subscribe(roomID string) {
	select {
	case b.subCmdCh <- subRequest{roomID: roomID, add: true}:
	case <-b.done:
	}
}

func (b *BrokerClient) Unsubscribe(roomID string) {
	select {
	case b.subCmdCh <- subRequest{roomID: roomID, add: false}:
	case <-b.done:
	}
}

func (b *BrokerClient) Messages() <-chan roomMessage {
	return b.recvCh
}

func (b *BrokerClient) Close() {
	close(b.done)
	b.wg.Wait()
	_ = b.ctx.Term()
	close(b.recvCh)
}

func (b *BrokerClient) pubLoop() {
	defer b.wg.Done()

	pub, err := b.ctx.NewSocket(zmq4.PUB)
	if err != nil {
		log.Printf("broker pub socket error: %v", err)
		return
	}
	defer pub.Close()

	if err := pub.Connect(b.pubAddr); err != nil {
		log.Printf("broker pub connect error addr=%s err=%v", b.pubAddr, err)
		return
	}

	for {
		select {
		case req := <-b.pubCh:
			wire := encodeWire(req.mt, req.data)
			if _, err := pub.SendMessage(req.roomID, wire); err != nil {
				log.Printf("broker publish error room=%s err=%v", req.roomID, err)
			}
		case <-b.done:
			return
		}
	}
}

func (b *BrokerClient) subLoop() {
	defer b.wg.Done()

	sub, err := b.ctx.NewSocket(zmq4.SUB)
	if err != nil {
		log.Printf("broker sub socket error: %v", err)
		return
	}
	defer sub.Close()

	if err := sub.Connect(b.subAddr); err != nil {
		log.Printf("broker sub connect error addr=%s err=%v", b.subAddr, err)
		return
	}

	poller := zmq4.NewPoller()
	poller.Add(sub, zmq4.POLLIN)
	subscribed := make(map[string]struct{})

	for {
		if !b.handleSubCommands(sub, subscribed) {
			return
		}

		events, err := poller.Poll(200 * time.Millisecond)
		if err != nil {
			if errors.Is(err, zmq4.Errno(zmq4.ETERM)) {
				return
			}
			log.Printf("broker sub poll error: %v", err)
			continue
		}
		if len(events) == 0 {
			select {
			case <-b.done:
				return
			default:
			}
			continue
		}

		frames, err := sub.RecvMessageBytes(0)
		if err != nil {
			if errors.Is(err, zmq4.Errno(zmq4.ETERM)) {
				return
			}
			log.Printf("broker recv error: %v", err)
			continue
		}
		if len(frames) < 2 {
			continue
		}

		mt, data := decodeWire(frames[1])
		msg := roomMessage{
			roomID: string(frames[0]),
			mt:     mt,
			data:   data,
		}

		select {
		case b.recvCh <- msg:
		case <-b.done:
			return
		default:
			log.Printf("broker recv dropped room=%s reason=queue_full", msg.roomID)
		}
	}
}

func (b *BrokerClient) handleSubCommands(sub *zmq4.Socket, subscribed map[string]struct{}) bool {
	for {
		select {
		case cmd := <-b.subCmdCh:
			if cmd.add {
				if _, ok := subscribed[cmd.roomID]; ok {
					continue
				}
				if err := sub.SetSubscribe(cmd.roomID); err != nil {
					log.Printf("broker subscribe error room=%s err=%v", cmd.roomID, err)
					continue
				}
				subscribed[cmd.roomID] = struct{}{}
				log.Printf("broker subscribed room=%s", cmd.roomID)
			} else {
				if _, ok := subscribed[cmd.roomID]; !ok {
					continue
				}
				if err := sub.SetUnsubscribe(cmd.roomID); err != nil {
					log.Printf("broker unsubscribe error room=%s err=%v", cmd.roomID, err)
					continue
				}
				delete(subscribed, cmd.roomID)
				log.Printf("broker unsubscribed room=%s", cmd.roomID)
			}
		case <-b.done:
			return false
		default:
			return true
		}
	}
}

type ConnectionManager struct {
	mu           sync.Mutex
	conns        map[*websocket.Conn]*connState
	rooms        map[string]map[*websocket.Conn]*connState
	broker       *BrokerClient
	listener     net.Listener
	server       *http.Server
	stopOnce     sync.Once
	forwarderWg  sync.WaitGroup
	forwarderSig chan struct{}
}

func NewConnectionManager(broker *BrokerClient) *ConnectionManager {
	m := &ConnectionManager{
		conns:        make(map[*websocket.Conn]*connState),
		rooms:        make(map[string]map[*websocket.Conn]*connState),
		broker:       broker,
		forwarderSig: make(chan struct{}),
	}

	m.forwarderWg.Add(1)
	go m.forwardBrokerMessages()
	return m
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
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
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
	roomID := roomFromRequest(r)

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = "unknown"
	}
	podIP := os.Getenv("POD_IP")
	if podIP == "" {
		podIP = "unknown"
	}
	log.Printf("ws connected pod=%s ip=%s remote=%s room=%s", podName, podIP, r.RemoteAddr, roomID)

	state := &connState{
		conn:   conn,
		roomID: roomID,
		send:   make(chan outbound, 16),
		done:   make(chan struct{}),
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

		if mt == websocket.TextMessage || mt == websocket.BinaryMessage {
			m.broker.Publish(state.roomID, mt, msg)
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
	roomConns := m.rooms[state.roomID]
	if roomConns == nil {
		roomConns = make(map[*websocket.Conn]*connState)
		m.rooms[state.roomID] = roomConns
	}
	needSubscribe := len(roomConns) == 0
	roomConns[state.conn] = state
	m.mu.Unlock()

	if needSubscribe {
		m.broker.Subscribe(state.roomID)
	}
}

func (m *ConnectionManager) removeConn(conn *websocket.Conn) {
	m.mu.Lock()
	state, ok := m.conns[conn]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.conns, conn)

	roomConns := m.rooms[state.roomID]
	needUnsubscribe := false
	if roomConns != nil {
		delete(roomConns, conn)
		if len(roomConns) == 0 {
			delete(m.rooms, state.roomID)
			needUnsubscribe = true
		}
	}
	m.mu.Unlock()

	if needUnsubscribe {
		m.broker.Unsubscribe(state.roomID)
	}
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

func (m *ConnectionManager) close() {
	close(m.forwarderSig)
	m.forwarderWg.Wait()
}

func (m *ConnectionManager) forwardBrokerMessages() {
	defer m.forwarderWg.Done()
	for {
		select {
		case msg, ok := <-m.broker.Messages():
			if !ok {
				return
			}
			m.broadcastRoom(msg)
		case <-m.forwarderSig:
			return
		}
	}
}

func (m *ConnectionManager) broadcastRoom(msg roomMessage) {
	m.mu.Lock()
	roomConns := m.rooms[msg.roomID]
	states := make([]*connState, 0, len(roomConns))
	for _, st := range roomConns {
		states = append(states, st)
	}
	m.mu.Unlock()

	for _, st := range states {
		select {
		case st.send <- outbound{mt: msg.mt, data: msg.data}:
		case <-st.done:
		default:
			log.Printf("drop outbound room=%s reason=conn_buffer_full", msg.roomID)
		}
	}
}

func roomFromRequest(r *http.Request) string {
	if room := strings.TrimSpace(r.URL.Query().Get("room")); room != "" {
		return room
	}

	path := strings.Trim(r.URL.Path, "/")
	if path == "" || path == "ws" {
		return lobbyRoom
	}
	if strings.HasPrefix(path, "ws/") {
		room := strings.TrimSpace(strings.TrimPrefix(path, "ws/"))
		if room != "" {
			return room
		}
	}
	return lobbyRoom
}

func encodeWire(mt int, data []byte) []byte {
	wire := make([]byte, len(data)+1)
	wire[0] = byte(mt)
	copy(wire[1:], data)
	return wire
}

func decodeWire(wire []byte) (int, []byte) {
	if len(wire) == 0 {
		return websocket.TextMessage, nil
	}
	mt := int(wire[0])
	if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
		mt = websocket.TextMessage
	}
	return mt, append([]byte(nil), wire[1:]...)
}

func main() {
	pubAddr := os.Getenv("ZMQ_BROKER_PUB_ADDR")
	if pubAddr == "" {
		pubAddr = "tcp://127.0.0.1:5556"
	}
	subAddr := os.Getenv("ZMQ_BROKER_SUB_ADDR")
	if subAddr == "" {
		subAddr = "tcp://127.0.0.1:5557"
	}

	broker, err := NewBrokerClient(pubAddr, subAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer broker.Close()

	manager := NewConnectionManager(broker)
	defer manager.close()

	if err := manager.Run(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Allow in-flight handlers to exit gracefully.
	_ = manager.server.Shutdown(context.Background())
}

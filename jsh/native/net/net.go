package net

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("CreateServer", CreateServer)
	m.Set("Connect", Connect)
	m.Set("Dial", Dial)
}

// CreateServer creates a new TCP server
func CreateServer(obj *goja.Object, dispatch engine.EventDispatchFunc) *Server {
	return &Server{
		obj:         obj,
		dispatch:    dispatch,
		connections: make(map[string]*Socket),
	}
}

type Server struct {
	obj         *goja.Object
	dispatch    engine.EventDispatchFunc
	listener    net.Listener
	connections map[string]*Socket
	mu          sync.Mutex
	listening   bool
	closed      bool
}

func (s *Server) emit(event string, data any) {
	if s.obj == nil {
		return
	}
	s.dispatch(s.obj, event, data)
}

func (s *Server) Listen(port int, host string, backlog int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listening {
		return fmt.Errorf("server is already listening")
	}

	if host == "" {
		host = "0.0.0.0"
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.listener = listener
	s.listening = true

	// Start accepting connections in a goroutine
	go s.acceptLoop()

	s.emit("listening", nil)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			s.emit("error", err)
			continue
		}

		// Create a Socket object for the accepted connection
		remoteAddr := conn.RemoteAddr().String()

		// Create socket with a dispatch-based emit function
		// This ensures all socket events go through the event loop
		socket := &Socket{
			obj:      nil, // Will be set later if needed
			conn:     conn,
			dispatch: s.dispatch,
			closed:   false,
		}

		// Add to server's connection map
		s.mu.Lock()
		s.connections[remoteAddr] = socket
		s.mu.Unlock()

		// Emit connection event with the socket object
		s.emit("accept", socket)
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.listening = false

	// Close all connections
	for _, socket := range s.connections {
		socket.Destroy()
	}
	s.connections = make(map[string]*Socket)

	if s.listener != nil {
		err := s.listener.Close()
		s.emit("close", nil)
		return err
	}

	return nil
}

func (s *Server) Address() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return nil
	}

	addr := s.listener.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return map[string]any{
			"address": addr.String(),
		}
	}

	return map[string]any{
		"address": tcpAddr.IP.String(),
		"port":    tcpAddr.Port,
		"family":  "IPv4",
	}
}

func (s *Server) GetConnections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.connections)
}

// Connect creates a new TCP client connection
func Connect(obj *goja.Object, port int, host string, dispatch engine.EventDispatchFunc) (*Socket, error) {
	if host == "" {
		host = "localhost"
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	socket := newSocketFromConn(conn, obj, dispatch)
	// Emit connect event after a brief delay to allow JavaScript to set up handlers
	socket.emit("connect", nil)

	return socket, nil
}

// Dial is an alias for Connect
func Dial(obj *goja.Object, port int, host string, dispatch engine.EventDispatchFunc) (*Socket, error) {
	return Connect(obj, port, host, dispatch)
}

// Socket represents a TCP socket connection
type Socket struct {
	obj          *goja.Object
	conn         net.Conn
	dispatch     engine.EventDispatchFunc
	mu           sync.Mutex
	closed       bool
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newSocketFromConn(conn net.Conn, obj *goja.Object, dispatch engine.EventDispatchFunc) *Socket {
	socket := &Socket{
		obj:      obj,
		conn:     conn,
		dispatch: dispatch,
		closed:   false,
	}

	// Start reading in background
	go socket.readLoop()

	return socket
}

func (s *Socket) readLoop() {
	buf := make([]byte, 8192)
	for {
		if s.readTimeout > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.readTimeout))
		}

		n, err := s.conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				s.emit("end", nil)
				s.Close()
				return
			}
			s.mu.Lock()
			if !s.closed {
				s.mu.Unlock()
				s.emit("error", err)
			} else {
				s.mu.Unlock()
			}
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			s.emit("data", data)
		}
	}
}

func (s *Socket) emit(event string, data any) {
	if s.obj == nil {
		return
	}
	s.dispatch(s.obj, event, data)
}

func (s *Socket) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, fmt.Errorf("socket is closed")
	}

	if s.writeTimeout > 0 {
		s.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
	}

	n, err := s.conn.Write(data)
	if err != nil {
		s.emit("error", err)
		return n, err
	}

	return n, nil
}

func (s *Socket) WriteString(str string, encoding string) (int, error) {
	return s.Write([]byte(str))
}

func (s *Socket) End(data []byte) error {
	if len(data) > 0 {
		_, err := s.Write(data)
		if err != nil {
			return err
		}
	}
	return s.Close()
}

func (s *Socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	var err error
	if s.conn != nil {
		err = s.conn.Close()
	}
	s.emit("close", false) // false = not an error close
	return err
}

func (s *Socket) Destroy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	var err error
	if s.conn != nil {
		err = s.conn.Close()
	}
	s.emit("close", true) // true = error close (destroyed)
	return err
}

func (s *Socket) SetTimeout(timeout int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readTimeout = time.Duration(timeout) * time.Millisecond
}

func (s *Socket) SetKeepAlive(enable bool, initialDelay int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(enable); err != nil {
			return err
		}
		if enable && initialDelay > 0 {
			return tcpConn.SetKeepAlivePeriod(time.Duration(initialDelay) * time.Millisecond)
		}
		return nil
	}

	return fmt.Errorf("not a TCP connection")
}

func (s *Socket) SetNoDelay(noDelay bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		return tcpConn.SetNoDelay(noDelay)
	}

	return fmt.Errorf("not a TCP connection")
}

func (s *Socket) Address() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}

	addr := s.conn.LocalAddr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return map[string]any{
			"address": addr.String(),
		}
	}

	return map[string]any{
		"address": tcpAddr.IP.String(),
		"port":    tcpAddr.Port,
		"family":  "IPv4",
	}
}

func (s *Socket) RemoteAddress() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}

	addr := s.conn.RemoteAddr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return map[string]any{
			"address": addr.String(),
		}
	}

	return map[string]any{
		"address": tcpAddr.IP.String(),
		"port":    tcpAddr.Port,
		"family":  "IPv4",
	}
}

func (s *Socket) LocalAddress() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return ""
	}

	return s.conn.LocalAddr().String()
}

func (s *Socket) RemoteAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return ""
	}

	return s.conn.RemoteAddr().String()
}

func (s *Socket) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Socket) SetObject(obj *goja.Object) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.obj = obj
}

func (s *Socket) StartReading() {
	// Start reading loop in background
	go s.readLoop()
}

func (s *Socket) Pause() {
	// In Go, we don't have a direct pause mechanism
	// This would need to be implemented with channels if needed
}

func (s *Socket) Resume() {
	// In Go, we don't have a direct resume mechanism
	// This would need to be implemented with channels if needed
}

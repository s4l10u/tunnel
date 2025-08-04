package tunnel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ForwarderConfig represents a port forwarding configuration
type ForwarderConfig struct {
	Name          string `yaml:"name"`
	Port          int    `yaml:"port"`
	Target        string `yaml:"target"`
	ClientID      string `yaml:"client_id"`
	Enabled       bool   `yaml:"enabled"`
	Description   string `yaml:"description"`
	WarningOnFail bool   `yaml:"warning_on_fail"`
}

// ImprovedServer handles WebSocket tunnel connections with improved reliability
type ImprovedServer struct {
	logger     *zap.Logger
	authToken  string
	clients    *ClientManager
	sessions   *SessionManager
	upgrader   websocket.Upgrader
	config     ServerConfig
	forwarders map[string]string // clientID -> target mapping
}

// ServerConfig holds server configuration
type ServerConfig struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PingInterval    time.Duration
	PongTimeout     time.Duration
	MaxMessageSize  int64
	SendBufferSize  int
	EnableHeartbeat bool
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    10 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     60 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1MB
		SendBufferSize:  512,
		EnableHeartbeat: true,
	}
}

// ClientManager handles client connections with thread-safe operations
type ClientManager struct {
	clients map[string]*ImprovedServerClient
	mu      sync.RWMutex
	logger  *zap.Logger
}

func NewClientManager(logger *zap.Logger) *ClientManager {
	return &ClientManager{
		clients: make(map[string]*ImprovedServerClient),
		logger:  logger,
	}
}

func (cm *ClientManager) Add(client *ImprovedServerClient) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clients[client.ID] = client
	cm.logger.Info("Client added", zap.String("clientID", client.ID))
}

func (cm *ClientManager) Remove(clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if client, exists := cm.clients[clientID]; exists {
		client.Close()
		delete(cm.clients, clientID)
		cm.logger.Info("Client removed", zap.String("clientID", clientID))
	}
}

func (cm *ClientManager) Get(clientID string) (*ImprovedServerClient, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	client, exists := cm.clients[clientID]
	return client, exists
}

// SessionManager handles TCP sessions with improved lifecycle management
type SessionManager struct {
	sessions map[string]*TCPSession
	mu       sync.RWMutex
	logger   *zap.Logger
}

func NewSessionManager(logger *zap.Logger) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*TCPSession),
		logger:   logger,
	}
}

// TCPSession represents a single TCP forwarding session
type TCPSession struct {
	ID         string
	ClientID   string
	Conn       net.Conn
	Target     string
	ctx        context.Context
	cancel     context.CancelFunc
	writeQueue chan []byte
	closed     atomic.Bool
	ready      chan struct{}  // Signals when client has connected to local service
	logger     *zap.Logger
}

func (sm *SessionManager) Create(sessionID, clientID, target string, conn net.Conn, logger *zap.Logger) *TCPSession {
	ctx, cancel := context.WithCancel(context.Background())
	session := &TCPSession{
		ID:         sessionID,
		ClientID:   clientID,
		Conn:       conn,
		Target:     target,
		ctx:        ctx,
		cancel:     cancel,
		writeQueue: make(chan []byte, 256),
		ready:      make(chan struct{}, 1),
		logger:     logger,
	}
	
	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()
	
	// Start write pump for the session
	go session.writePump()
	
	return session
}

func (sm *SessionManager) Get(sessionID string) (*TCPSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if session, exists := sm.sessions[sessionID]; exists {
		session.Close()
		delete(sm.sessions, sessionID)
		sm.logger.Debug("Session removed", zap.String("sessionID", sessionID))
	}
}

// Close safely closes the TCP session
func (s *TCPSession) Close() {
	if s.closed.CompareAndSwap(false, true) {
		s.cancel()
		close(s.writeQueue)
		
		// Close ready channel if not already closed
		select {
		case <-s.ready:
		default:
			close(s.ready)
		}
		
		s.Conn.Close()
		s.logger.Debug("Session closed", zap.String("sessionID", s.ID))
	}
}

// Write queues data for writing to the TCP connection
func (s *TCPSession) Write(data []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("session closed")
	}
	
	select {
	case s.writeQueue <- data:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("session context cancelled")
	default:
		return fmt.Errorf("write queue full")
	}
}

// writePump handles writing data to the TCP connection
func (s *TCPSession) writePump() {
	defer s.Conn.Close()
	
	for {
		select {
		case data, ok := <-s.writeQueue:
			if !ok {
				return
			}
			
			s.Conn.SetWriteDeadline(time.Now().Add(1 * time.Minute))
			if _, err := s.Conn.Write(data); err != nil {
				s.logger.Error("TCP write error", zap.String("sessionID", s.ID), zap.Error(err))
				return
			}
			
		case <-s.ctx.Done():
			return
		}
	}
}

// ImprovedServerClient represents a connected tunnel client
type ImprovedServerClient struct {
	ID          string
	Conn        *websocket.Conn
	Send        chan []byte
	server      *ImprovedServer
	lastPing    atomic.Int64
	ctx         context.Context
	cancel      context.CancelFunc
	closeOnce   sync.Once
}

// NewImprovedServer creates a new improved tunnel server
func NewImprovedServer(logger *zap.Logger, authToken string, forwarders []ForwarderConfig) *ImprovedServer {
	config := DefaultServerConfig()
	
	// Build forwarder mapping from clientID to target
	forwarderMap := make(map[string]string)
	for _, fw := range forwarders {
		if fw.Enabled {
			forwarderMap[fw.ClientID] = fw.Target
		}
	}
	
	return &ImprovedServer{
		logger:     logger,
		authToken:  authToken,
		clients:    NewClientManager(logger),
		sessions:   NewSessionManager(logger),
		config:     config,
		forwarders: forwarderMap,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Configure based on security needs
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// HandleTunnel handles WebSocket tunnel connections
func (s *ImprovedServer) HandleTunnel(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	token := r.Header.Get("Authorization")
	if token != "Bearer "+s.authToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", time.Now().Unix())
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	client := &ImprovedServerClient{
		ID:     clientID,
		Conn:   conn,
		Send:   make(chan []byte, s.config.SendBufferSize),
		server: s,
		ctx:    ctx,
		cancel: cancel,
	}
	
	client.lastPing.Store(time.Now().Unix())

	// Configure WebSocket connection
	conn.SetReadLimit(s.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.config.PongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.config.PongTimeout))
		client.lastPing.Store(time.Now().Unix())
		return nil
	})

	s.clients.Add(client)

	// Start goroutines
	go client.writePump()
	go client.readPump()
	
	if s.config.EnableHeartbeat {
		go client.heartbeat()
	}
}

// Close safely closes the client connection
func (c *ImprovedServerClient) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.Send)
		c.Conn.Close()
		c.server.logger.Info("Client connection closed", zap.String("clientID", c.ID))
	})
}

// readPump handles reading messages from the client
func (c *ImprovedServerClient) readPump() {
	defer func() {
		c.server.clients.Remove(c.ID)
	}()

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.server.logger.Error("WebSocket error", zap.String("clientID", c.ID), zap.Error(err))
			}
			break
		}

		select {
		case <-c.ctx.Done():
			return
		default:
			c.server.handleMessage(c, &msg)
		}
	}
}

// writePump handles writing messages to the client
func (c *ImprovedServerClient) writePump() {
	ticker := time.NewTicker(c.server.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(c.server.config.WriteTimeout))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.server.logger.Error("Write error", zap.String("clientID", c.ID), zap.Error(err))
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(c.server.config.WriteTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// heartbeat monitors client connection health
func (c *ImprovedServerClient) heartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lastPing := time.Unix(c.lastPing.Load(), 0)
			if time.Since(lastPing) > 2*c.server.config.PongTimeout {
				c.server.logger.Warn("Client heartbeat timeout", zap.String("clientID", c.ID))
				c.server.clients.Remove(c.ID)
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// StartTCPForwarder starts a TCP forwarder for a specific port
func (s *ImprovedServer) StartTCPForwarder(port int, clientID string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to start TCP forwarder on port %d: %w", port, err)
	}

	go func() {
		defer listener.Close()
		s.logger.Info("TCP forwarder started", zap.Int("port", port), zap.String("clientID", clientID))

		for {
			conn, err := listener.Accept()
			if err != nil {
				s.logger.Error("Accept failed", zap.Error(err))
				continue
			}

			go s.handleTCPConnection(conn, clientID, port)
		}
	}()

	return nil
}

// handleTCPConnection handles an incoming TCP connection
func (s *ImprovedServer) handleTCPConnection(conn net.Conn, clientID string, remotePort int) {
	sessionID := fmt.Sprintf("%s-%d-%d", clientID, remotePort, time.Now().UnixNano())
	
	// Determine target from forwarder configuration
	target, exists := s.forwarders[clientID]
	if !exists {
		// Fallback to environment variables for backward compatibility, then localhost
		switch clientID {
		case "airgap-web":
			target = getEnvOrDefault("TARGET_WEB", "webapp:80")
		case "airgap-db":
			target = getEnvOrDefault("TARGET_DB", "database:5432")
		case "airgap-ssh":
			target = getEnvOrDefault("TARGET_SSH", "ssh-server:22")
		case "airgap-mongodb":
			target = getEnvOrDefault("TARGET_MONGODB", "mongodb:27017")
		default:
			target = fmt.Sprintf("localhost:%d", remotePort)
		}
	}

	// Get the client
	client, exists := s.clients.Get(clientID)
	if !exists {
		s.logger.Warn("Client not found for TCP connection", zap.String("clientID", clientID))
		conn.Close()
		return
	}

	// Create session
	session := s.sessions.Create(sessionID, clientID, target, conn, s.logger)
	
	s.logger.Info("Starting TCP session", 
		zap.String("sessionID", sessionID),
		zap.String("clientID", clientID),
		zap.String("target", target))

	// Send connect message to client
	connectMsg := ForwardMessage{
		Type:      "connect",
		SessionID: sessionID,
		Target:    target,
	}

	if err := s.sendForwardMessageToClient(client, connectMsg); err != nil {
		s.logger.Error("Failed to send connect message", zap.Error(err))
		s.sessions.Remove(sessionID)
		return
	}

	// Wait for client to confirm connection before starting to read data
	select {
	case <-session.ready:
		s.logger.Info("Client confirmed connection, starting data flow", zap.String("sessionID", sessionID))
		// Start reading from the TCP connection
		go s.readFromTCPConnection(session, client)
	case <-session.ctx.Done():
		s.logger.Info("Session cancelled before client connected", zap.String("sessionID", sessionID))
		s.sessions.Remove(sessionID)
		return
	case <-time.After(10 * time.Second):
		s.logger.Warn("Timeout waiting for client connection", zap.String("sessionID", sessionID))
		s.sessions.Remove(sessionID)
		return
	}
	
	// Wait for session to complete
	<-session.ctx.Done()
	s.sessions.Remove(sessionID)
}

// readFromTCPConnection reads data from external TCP connection and forwards to client
func (s *ImprovedServer) readFromTCPConnection(session *TCPSession, client *ImprovedServerClient) {
	s.logger.Info("Starting to read from TCP connection", zap.String("sessionID", session.ID))
	defer func() {
		s.logger.Info("Stopping TCP connection reader", zap.String("sessionID", session.ID))
		// Send disconnect to client
		disconnectMsg := ForwardMessage{
			Type:      "disconnect",
			SessionID: session.ID,
		}
		s.sendForwardMessageToClient(client, disconnectMsg)
		session.Close()
	}()

	buffer := make([]byte, 32*1024)
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
		}

		session.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := session.Conn.Read(buffer)
		if err != nil {
			if err != io.EOF && err != io.ErrClosedPipe {
				s.logger.Info("TCP read error", zap.String("sessionID", session.ID), zap.Error(err))
			} else {
				s.logger.Info("TCP connection closed", zap.String("sessionID", session.ID), zap.Error(err))
			}
			return
		}
		
		s.logger.Info("Read data from TCP connection", zap.String("sessionID", session.ID), zap.Int("bytes", n))

		dataMsg := ForwardMessage{
			Type:      "data",
			SessionID: session.ID,
			Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		if err := s.sendForwardMessageToClient(client, dataMsg); err != nil {
			s.logger.Error("Failed to send data to client", zap.Error(err))
			return
		}
		
		s.logger.Info("Sent data to client", zap.String("sessionID", session.ID), zap.Int("bytes", n))
	}
}

// handleMessage handles incoming messages from clients
func (s *ImprovedServer) handleMessage(client *ImprovedServerClient, msg *Message) {
	switch msg.Type {
	case "register":
		response := Message{
			Type: "registered",
			ID:   client.ID,
		}
		s.sendMessageToClient(client, response)

	case "forward":
		s.handleForwardMessage(client, msg.Data)

	case "ping":
		response := Message{Type: "pong"}
		s.sendMessageToClient(client, response)

	default:
		s.logger.Warn("Unknown message type", zap.String("type", msg.Type))
	}
}

// handleForwardMessage handles forward messages from clients
func (s *ImprovedServer) handleForwardMessage(client *ImprovedServerClient, data json.RawMessage) {
	var msg ForwardMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Error("Failed to unmarshal forward message", zap.Error(err))
		return
	}

	switch msg.Type {
	case "connected":
		s.logger.Info("Client connected to local service", zap.String("sessionID", msg.SessionID))
		
		// Signal that the client is ready to receive data
		session, exists := s.sessions.Get(msg.SessionID)
		if exists {
			select {
			case session.ready <- struct{}{}:
				s.logger.Info("Signaled session ready", zap.String("sessionID", msg.SessionID))
			default:
				// Channel already closed or full
			}
		}

	case "data":
		// Data from client to forward to external connection
		session, exists := s.sessions.Get(msg.SessionID)
		if !exists {
			s.logger.Warn("Session not found for data", zap.String("sessionID", msg.SessionID))
			return
		}

		data, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			s.logger.Error("Failed to decode data", zap.Error(err))
			return
		}

		if err := session.Write(data); err != nil {
			s.logger.Error("Failed to write to TCP connection", zap.Error(err))
			s.sessions.Remove(msg.SessionID)
		}

	case "disconnect":
		s.logger.Info("Client disconnecting session", zap.String("sessionID", msg.SessionID))
		s.sessions.Remove(msg.SessionID)

	case "error":
		s.logger.Error("Client error", 
			zap.String("sessionID", msg.SessionID),
			zap.String("error", msg.Error))
		s.sessions.Remove(msg.SessionID)
	}
}

// sendMessageToClient sends a message to a client
func (s *ImprovedServer) sendMessageToClient(client *ImprovedServerClient, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case client.Send <- data:
		return nil
	case <-client.ctx.Done():
		return fmt.Errorf("client context cancelled")
	default:
		// Check if client is still connected
		if _, exists := s.clients.Get(client.ID); !exists {
			return fmt.Errorf("client disconnected")
		}
		return fmt.Errorf("client send buffer full")
	}
}

// sendForwardMessageToClient sends a forward message to a client
func (s *ImprovedServer) sendForwardMessageToClient(client *ImprovedServerClient, msg ForwardMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	wrappedMsg := Message{
		Type: "forward",
		Data: data,
	}

	return s.sendMessageToClient(client, wrappedMsg)
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
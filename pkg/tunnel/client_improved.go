package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ImprovedClientConfig holds configuration for the improved client
type ImprovedClientConfig struct {
	ServerURL          string
	AuthToken          string
	ClientID           string
	SkipVerify         bool
	Logger             *zap.Logger
	ReconnectInterval  time.Duration
	MaxReconnectDelay  time.Duration
	PingInterval       time.Duration
	PongTimeout        time.Duration
	WriteTimeout       time.Duration
	ReadBufferSize     int
	WriteBufferSize    int
	EnableCompression  bool
}

// DefaultImprovedClientConfig returns default client configuration
func DefaultImprovedClientConfig(serverURL, authToken, clientID string, logger *zap.Logger) ImprovedClientConfig {
	return ImprovedClientConfig{
		ServerURL:         serverURL,
		AuthToken:         authToken,
		ClientID:          clientID,
		Logger:            logger,
		ReconnectInterval: 5 * time.Second,
		MaxReconnectDelay: 2 * time.Minute,
		PingInterval:      30 * time.Second,
		PongTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadBufferSize:    1024 * 1024, // 1MB
		WriteBufferSize:   1024 * 1024, // 1MB
		EnableCompression: true,
	}
}

// ImprovedClient represents an improved tunnel client with better reliability
type ImprovedClient struct {
	config          ImprovedClientConfig
	conn            *websocket.Conn
	connMu          sync.RWMutex
	send            chan []byte
	sessions        *ClientSessionManager
	forwarders      map[string]*PortForwarder
	forwardersMu    sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	reconnectDelay  time.Duration
	isConnected     atomic.Bool
	lastError       atomic.Value
	metrics         *ClientMetrics
}

// ClientMetrics tracks client performance metrics
type ClientMetrics struct {
	connectAttempts   atomic.Int64
	successfulConnects atomic.Int64
	messagesSent      atomic.Int64
	messagesReceived  atomic.Int64
	bytesTransferred  atomic.Int64
	activeSessions    atomic.Int32
}

// ClientSessionManager manages client sessions with improved lifecycle
type ClientSessionManager struct {
	sessions map[string]*ClientSession
	mu       sync.RWMutex
	logger   *zap.Logger
}

// ClientSession represents a client-side forwarding session
type ClientSession struct {
	ID         string
	LocalConn  net.Conn
	RemoteAddr string
	ctx        context.Context
	cancel     context.CancelFunc
	writeQueue chan []byte
	closed     atomic.Bool
	client     *ImprovedClient
	logger     *zap.Logger
}

// PortForwarder manages port forwarding configuration
type PortForwarder struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewImprovedClient creates a new improved tunnel client
func NewImprovedClient(config ImprovedClientConfig) *ImprovedClient {
	if config.ClientID == "" {
		config.ClientID = fmt.Sprintf("airgap-%d", time.Now().Unix())
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &ImprovedClient{
		config:         config,
		send:           make(chan []byte, 512),
		sessions:       NewClientSessionManager(config.Logger),
		forwarders:     make(map[string]*PortForwarder),
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: config.ReconnectInterval,
		metrics:        &ClientMetrics{},
	}

	return client
}

// NewClientSessionManager creates a new session manager
func NewClientSessionManager(logger *zap.Logger) *ClientSessionManager {
	return &ClientSessionManager{
		sessions: make(map[string]*ClientSession),
		logger:   logger,
	}
}

// Start starts the improved client with automatic reconnection
func (c *ImprovedClient) Start(ctx context.Context) error {
	// Merge contexts
	clientCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-c.ctx.Done():
			cancel()
		case <-clientCtx.Done():
			c.cancel()
		}
	}()

	// Connection loop with exponential backoff
	for {
		select {
		case <-clientCtx.Done():
			return clientCtx.Err()
		default:
		}

		c.metrics.connectAttempts.Add(1)
		
		err := c.connect(clientCtx)
		if err != nil {
			c.lastError.Store(err)
			c.config.Logger.Error("Connection failed", 
				zap.Error(err),
				zap.Duration("nextRetry", c.reconnectDelay))
			
			select {
			case <-clientCtx.Done():
				return clientCtx.Err()
			case <-time.After(c.reconnectDelay):
				// Exponential backoff
				c.reconnectDelay = c.reconnectDelay * 2
				if c.reconnectDelay > c.config.MaxReconnectDelay {
					c.reconnectDelay = c.config.MaxReconnectDelay
				}
			}
			continue
		}

		// Reset delay on successful connection
		c.reconnectDelay = c.config.ReconnectInterval
		c.metrics.successfulConnects.Add(1)
		
		// Wait for disconnection
		<-c.waitForDisconnect()
		
		c.config.Logger.Info("Connection lost, attempting to reconnect...")
		time.Sleep(1 * time.Second)
	}
}

// connect establishes a WebSocket connection to the server
func (c *ImprovedClient) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.config.SkipVerify,
		},
		ReadBufferSize:    c.config.ReadBufferSize,
		WriteBufferSize:   c.config.WriteBufferSize,
		EnableCompression: c.config.EnableCompression,
		HandshakeTimeout:  10 * time.Second,
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.config.AuthToken)
	header.Set("X-Client-ID", c.config.ClientID)

	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	c.config.Logger.Info("Connecting to server", zap.String("url", u.String()))

	conn, _, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	// Configure connection
	conn.SetReadLimit(int64(c.config.ReadBufferSize))
	conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
		return nil
	})

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.isConnected.Store(true)

	// Send registration message
	regMsg := Message{
		Type: "register",
		ID:   c.config.ClientID,
	}
	if err := c.sendMessage(regMsg); err != nil {
		conn.Close()
		return fmt.Errorf("registration failed: %w", err)
	}

	c.config.Logger.Info("Connected and registered", zap.String("clientID", c.config.ClientID))

	// Start connection handlers
	go c.readPump()
	go c.writePump()
	go c.keepAlive(ctx)

	return nil
}

// waitForDisconnect returns a channel that closes when disconnected
func (c *ImprovedClient) waitForDisconnect() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for c.isConnected.Load() {
			time.Sleep(100 * time.Millisecond)
		}
		close(ch)
	}()
	return ch
}

// AddPortForwarder adds a port forwarder
func (c *ImprovedClient) AddPortForwarder(localPort int, remoteHost string, remotePort int) error {
	key := fmt.Sprintf("%d:%s:%d", localPort, remoteHost, remotePort)
	
	c.forwardersMu.Lock()
	defer c.forwardersMu.Unlock()
	
	if _, exists := c.forwarders[key]; exists {
		return fmt.Errorf("forwarder already exists for %s", key)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", localPort, err)
	}

	ctx, cancel := context.WithCancel(c.ctx)
	
	forwarder := &PortForwarder{
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		listener:   listener,
		ctx:        ctx,
		cancel:     cancel,
	}

	c.forwarders[key] = forwarder

	// Start accepting connections
	go c.acceptConnections(forwarder)

	c.config.Logger.Info("Added port forwarder",
		zap.Int("localPort", localPort),
		zap.String("remoteHost", remoteHost),
		zap.Int("remotePort", remotePort))

	return nil
}

// acceptConnections accepts incoming connections for a forwarder
func (c *ImprovedClient) acceptConnections(forwarder *PortForwarder) {
	defer forwarder.listener.Close()

	for {
		select {
		case <-forwarder.ctx.Done():
			return
		default:
		}

		conn, err := forwarder.listener.Accept()
		if err != nil {
			if !isTemporaryError(err) {
				c.config.Logger.Error("Accept failed", zap.Error(err))
				return
			}
			continue
		}

		// Handle connection
		go c.handleLocalConnection(conn, forwarder.RemoteHost, forwarder.RemotePort)
	}
}

// handleLocalConnection handles a local connection that needs forwarding
func (c *ImprovedClient) handleLocalConnection(conn net.Conn, remoteHost string, remotePort int) {
	sessionID := fmt.Sprintf("%s-%d", c.config.ClientID, time.Now().UnixNano())
	target := fmt.Sprintf("%s:%d", remoteHost, remotePort)

	// Create session
	session := c.sessions.Create(sessionID, conn, target, c)
	defer c.sessions.Remove(sessionID)

	c.metrics.activeSessions.Add(1)
	defer c.metrics.activeSessions.Add(-1)

	// Send connect message
	msg := ForwardMessage{
		Type:      "connect",
		SessionID: sessionID,
		Target:    target,
	}

	if err := c.sendForwardMessage(msg); err != nil {
		c.config.Logger.Error("Failed to send connect message", zap.Error(err))
		return
	}

	// Start reading from local connection
	session.Start()
}

// Create creates a new client session
func (csm *ClientSessionManager) Create(sessionID string, conn net.Conn, target string, client *ImprovedClient) *ClientSession {
	ctx, cancel := context.WithCancel(context.Background())
	
	session := &ClientSession{
		ID:         sessionID,
		LocalConn:  conn,
		RemoteAddr: target,
		ctx:        ctx,
		cancel:     cancel,
		writeQueue: make(chan []byte, 256),
		client:     client,
		logger:     client.config.Logger,
	}

	csm.mu.Lock()
	csm.sessions[sessionID] = session
	csm.mu.Unlock()

	return session
}

// Get retrieves a session by ID
func (csm *ClientSessionManager) Get(sessionID string) (*ClientSession, bool) {
	csm.mu.RLock()
	defer csm.mu.RUnlock()
	session, exists := csm.sessions[sessionID]
	return session, exists
}

// Remove removes a session
func (csm *ClientSessionManager) Remove(sessionID string) {
	csm.mu.Lock()
	defer csm.mu.Unlock()
	
	if session, exists := csm.sessions[sessionID]; exists {
		session.Close()
		delete(csm.sessions, sessionID)
	}
}

// Start starts the session handlers
func (s *ClientSession) Start() {
	go s.readFromLocal()
	go s.writePump()
	
	// Wait for session to end
	<-s.ctx.Done()
}

// Close safely closes the session
func (s *ClientSession) Close() {
	if s.closed.CompareAndSwap(false, true) {
		s.cancel()
		close(s.writeQueue)
		s.LocalConn.Close()
		
		// Send disconnect message
		msg := ForwardMessage{
			Type:      "disconnect",
			SessionID: s.ID,
		}
		s.client.sendForwardMessage(msg)
	}
}

// Write queues data for writing to local connection
func (s *ClientSession) Write(data []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("session closed")
	}

	select {
	case s.writeQueue <- data:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("session cancelled")
	default:
		return fmt.Errorf("write queue full")
	}
}

// readFromLocal reads from local connection and forwards to server
func (s *ClientSession) readFromLocal() {
	defer s.Close()

	buffer := make([]byte, 32*1024)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		s.LocalConn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := s.LocalConn.Read(buffer)
		if err != nil {
			if err != io.EOF && !isTemporaryError(err) {
				s.logger.Debug("Local read error", zap.Error(err))
			}
			return
		}

		s.client.metrics.bytesTransferred.Add(int64(n))

		msg := ForwardMessage{
			Type:      "data",
			SessionID: s.ID,
			Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		if err := s.client.sendForwardMessage(msg); err != nil {
			s.logger.Error("Failed to forward data", zap.Error(err))
			return
		}
	}
}

// writePump writes queued data to local connection
func (s *ClientSession) writePump() {
	defer s.LocalConn.Close()

	for {
		select {
		case data, ok := <-s.writeQueue:
			if !ok {
				return
			}

			s.LocalConn.SetWriteDeadline(time.Now().Add(1 * time.Minute))
			if _, err := s.LocalConn.Write(data); err != nil {
				s.logger.Error("Local write error", zap.Error(err))
				return
			}

		case <-s.ctx.Done():
			return
		}
	}
}

// readPump reads messages from the server
func (c *ImprovedClient) readPump() {
	defer func() {
		c.isConnected.Store(false)
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.connMu.Unlock()
	}()

	for {
		var msg Message
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			return
		}

		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.config.Logger.Error("WebSocket error", zap.Error(err))
			}
			return
		}

		c.metrics.messagesReceived.Add(1)
		c.handleMessage(&msg)
	}
}

// writePump writes messages to the server
func (c *ImprovedClient) writePump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.connMu.Unlock()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil || !ok {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.config.Logger.Error("Write error", zap.Error(err))
				return
			}
			c.metrics.messagesSent.Add(1)

		case <-ticker.C:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// keepAlive sends periodic ping messages
func (c *ImprovedClient) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.sendMessage(Message{Type: "ping"}); err != nil {
				c.config.Logger.Error("Failed to send ping", zap.Error(err))
				return
			}
		case <-ctx.Done():
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// handleMessage handles incoming messages from server
func (c *ImprovedClient) handleMessage(msg *Message) {
	switch msg.Type {
	case "registered":
		c.config.Logger.Info("Registration confirmed")

	case "pong":
		// Keepalive response

	case "forward":
		c.handleForwardMessage(msg.Data)

	default:
		c.config.Logger.Warn("Unknown message type", zap.String("type", msg.Type))
	}
}

// handleForwardMessage handles forward messages from server
func (c *ImprovedClient) handleForwardMessage(data json.RawMessage) {
	var msg ForwardMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.config.Logger.Error("Failed to unmarshal forward message", zap.Error(err))
		return
	}

	switch msg.Type {
	case "connect":
		go c.handleRemoteConnect(&msg)

	case "data":
		c.handleRemoteData(&msg)

	case "disconnect":
		c.handleRemoteDisconnect(&msg)

	case "error":
		c.config.Logger.Error("Forward error",
			zap.String("sessionID", msg.SessionID),
			zap.String("error", msg.Error))
		c.sessions.Remove(msg.SessionID)
	}
}

// handleRemoteConnect handles connection request from server
func (c *ImprovedClient) handleRemoteConnect(msg *ForwardMessage) {
	// Connect to local service
	conn, err := net.DialTimeout("tcp", msg.Target, 10*time.Second)
	if err != nil {
		c.config.Logger.Error("Failed to connect to local service",
			zap.String("target", msg.Target),
			zap.Error(err))

		// Send error back
		errMsg := ForwardMessage{
			Type:      "error",
			SessionID: msg.SessionID,
			Error:     err.Error(),
		}
		c.sendForwardMessage(errMsg)
		return
	}

	// Create session
	session := c.sessions.Create(msg.SessionID, conn, msg.Target, c)

	c.config.Logger.Info("Connected to local service",
		zap.String("sessionID", msg.SessionID),
		zap.String("target", msg.Target))

	// Send success response
	successMsg := ForwardMessage{
		Type:      "connected",
		SessionID: msg.SessionID,
	}
	c.sendForwardMessage(successMsg)

	// Start session
	go session.Start()
}

// handleRemoteData handles data from server
func (c *ImprovedClient) handleRemoteData(msg *ForwardMessage) {
	c.config.Logger.Info("Received data from server", zap.String("sessionID", msg.SessionID), zap.Int("dataLen", len(msg.Data)))
	
	session, exists := c.sessions.Get(msg.SessionID)
	if !exists {
		c.config.Logger.Warn("Session not found for data", zap.String("sessionID", msg.SessionID))
		return
	}

	data, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		c.config.Logger.Error("Failed to decode data", zap.Error(err))
		return
	}

	c.config.Logger.Info("Decoded data", zap.String("sessionID", msg.SessionID), zap.Int("bytes", len(data)))
	c.metrics.bytesTransferred.Add(int64(len(data)))

	if err := session.Write(data); err != nil {
		c.config.Logger.Error("Failed to write to local service", zap.Error(err))
		c.sessions.Remove(msg.SessionID)
	} else {
		c.config.Logger.Info("Wrote data to local service", zap.String("sessionID", msg.SessionID), zap.Int("bytes", len(data)))
	}
}

// handleRemoteDisconnect handles disconnect from server
func (c *ImprovedClient) handleRemoteDisconnect(msg *ForwardMessage) {
	c.sessions.Remove(msg.SessionID)
}

// sendMessage sends a message to the server
func (c *ImprovedClient) sendMessage(msg Message) error {
	if !c.isConnected.Load() {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("client shutting down")
	default:
		return fmt.Errorf("send channel full")
	}
}

// sendForwardMessage sends a forward message to the server
func (c *ImprovedClient) sendForwardMessage(msg ForwardMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	wrappedMsg := Message{
		Type: "forward",
		Data: data,
	}

	return c.sendMessage(wrappedMsg)
}

// GetMetrics returns current client metrics
func (c *ImprovedClient) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"connectAttempts":    c.metrics.connectAttempts.Load(),
		"successfulConnects": c.metrics.successfulConnects.Load(),
		"messagesSent":       c.metrics.messagesSent.Load(),
		"messagesReceived":   c.metrics.messagesReceived.Load(),
		"bytesTransferred":   c.metrics.bytesTransferred.Load(),
		"activeSessions":     c.metrics.activeSessions.Load(),
		"isConnected":        c.isConnected.Load(),
	}
}

// CreateTunnelSession creates a new tunnel session that appears as a net.Conn
// This method allows the MongoDB proxy to create connections through the tunnel
func (c *ImprovedClient) CreateTunnelSession(target string) (net.Conn, error) {
	if !c.isConnected.Load() {
		return nil, fmt.Errorf("tunnel client not connected")
	}

	// Create a pipe for bidirectional communication
	clientConn, proxyConn := net.Pipe()
	
	// Generate unique session ID
	sessionID := fmt.Sprintf("%s-%d", c.config.ClientID, time.Now().UnixNano())
	
	// Create session to manage this connection
	session := c.sessions.Create(sessionID, clientConn, target, c)
	
	// Send connect message to server to request connection to target
	msg := ForwardMessage{
		Type:      "connect",
		SessionID: sessionID,
		Target:    target,
	}
	
	if err := c.sendForwardMessage(msg); err != nil {
		clientConn.Close()
		proxyConn.Close()
		c.sessions.Remove(sessionID)
		return nil, fmt.Errorf("failed to send connect message: %w", err)
	}
	
	c.config.Logger.Info("Created tunnel session", 
		zap.String("sessionID", sessionID),
		zap.String("target", target))
	
	// Start session handlers in background
	go session.Start()
	
	// Return the proxy end of the pipe that MongoDB proxy can use
	return proxyConn, nil
}

// isTemporaryError checks if an error is temporary
func isTemporaryError(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Temporary() || ne.Timeout()
	}
	return false
}
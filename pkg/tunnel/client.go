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
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type ClientConfig struct {
	ServerURL  string
	AuthToken  string
	ClientID   string
	SkipVerify bool
	Logger     *zap.Logger
}

type Client struct {
	config      ClientConfig
	conn        *websocket.Conn
	send        chan []byte
	forwarders  map[string]*Forwarder
	sessions    map[string]*Session
	mu          sync.RWMutex
	reconnectCh chan struct{}
}

type Forwarder struct {
	LocalPort  int
	RemotePort int
	Target     string
}

func NewClient(config ClientConfig) *Client {
	if config.ClientID == "" {
		config.ClientID = fmt.Sprintf("airgap-%d", time.Now().Unix())
	}
	
	return &Client{
		config:      config,
		send:        make(chan []byte, 256),
		forwarders:  make(map[string]*Forwarder),
		sessions:    make(map[string]*Session),
		reconnectCh: make(chan struct{}, 1),
	}
}

func (c *Client) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.connect(ctx); err != nil {
				c.config.Logger.Error("Connection failed", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			
			// Connection closed, try to reconnect
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-c.reconnectCh:
				c.config.Logger.Info("Reconnecting...")
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.config.SkipVerify,
		},
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.config.AuthToken)
	header.Set("X-Client-ID", c.config.ClientID)

	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	c.config.Logger.Info("Connecting to server", zap.String("url", u.String()))
	
	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	
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

	// Start pumps
	go c.readPump()
	go c.writePump()
	
	// Keep alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.sendMessage(Message{Type: "ping"}); err != nil {
				return err
			}
		case <-c.reconnectCh:
			return nil
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.config.Logger.Error("WebSocket error", zap.Error(err))
			}
			break
		}

		c.handleMessage(&msg)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case "registered":
		c.config.Logger.Info("Registration confirmed")
		
	case "pong":
		// Keepalive response
		
	case "forward":
		// Handle forwarding messages from server
		c.handleForwardMessage(msg.Data)
		
	default:
		c.config.Logger.Warn("Unknown message type", zap.String("type", msg.Type))
	}
}

func (c *Client) sendMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

func (c *Client) handleForwardMessage(data json.RawMessage) {
	var msg ForwardMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.config.Logger.Error("Failed to unmarshal forward message", zap.Error(err))
		return
	}

	switch msg.Type {
	case "connect":
		// Server wants to connect to a local service
		go c.handleRemoteConnect(&msg)
		
	case "data":
		// Data from server to forward to local service
		c.handleRemoteData(&msg)
		
	case "disconnect":
		// Close connection to local service
		c.handleRemoteDisconnect(&msg)
		
	case "error":
		c.config.Logger.Error("Forward error", 
			zap.String("sessionID", msg.SessionID),
			zap.String("error", msg.Error))
	}
}

func (c *Client) handleRemoteConnect(msg *ForwardMessage) {
	// Connect to local service
	conn, err := net.Dial("tcp", msg.Target)
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

	c.config.Logger.Info("Connected to local service",
		zap.String("sessionID", msg.SessionID),
		zap.String("target", msg.Target))

	session := &Session{
		ID:         msg.SessionID,
		LocalConn:  conn,
		RemoteAddr: msg.Target,
		Client:     c,
	}

	c.mu.Lock()
	c.sessions[msg.SessionID] = session
	c.mu.Unlock()

	// Send success response
	successMsg := ForwardMessage{
		Type:      "connected",
		SessionID: msg.SessionID,
	}
	c.sendForwardMessage(successMsg)

	// Start reading from local service
	go c.readFromLocal(session)
}

func (c *Client) handleRemoteData(msg *ForwardMessage) {
	c.mu.RLock()
	session, exists := c.sessions[msg.SessionID]
	c.mu.RUnlock()

	if !exists {
		return
	}

	data, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		c.config.Logger.Error("Failed to decode data", zap.Error(err))
		return
	}

	if _, err := session.LocalConn.Write(data); err != nil {
		c.config.Logger.Error("Failed to write to local service", zap.Error(err))
		c.handleRemoteDisconnect(msg)
	}
}

func (c *Client) handleRemoteDisconnect(msg *ForwardMessage) {
	c.mu.Lock()
	session, exists := c.sessions[msg.SessionID]
	if exists {
		delete(c.sessions, msg.SessionID)
	}
	c.mu.Unlock()

	if exists && session.LocalConn != nil {
		session.LocalConn.Close()
	}
}

func (c *Client) readFromLocal(session *Session) {
	defer func() {
		c.config.Logger.Info("Cleaning up local session", zap.String("sessionID", session.ID))
		c.mu.Lock()
		delete(c.sessions, session.ID)
		c.mu.Unlock()
		session.LocalConn.Close()
		
		// Send disconnect to server
		msg := ForwardMessage{
			Type:      "disconnect",
			SessionID: session.ID,
		}
		c.sendForwardMessage(msg)
	}()

	c.config.Logger.Info("Starting to read from local service", zap.String("sessionID", session.ID))

	buffer := make([]byte, 32*1024)
	for {
		// Set read timeout to keep connection alive
		session.LocalConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := session.LocalConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				c.config.Logger.Debug("Local connection read error", 
					zap.String("sessionID", session.ID), 
					zap.Error(err))
			}
			break
		}

		c.config.Logger.Debug("Read data from local service", 
			zap.String("sessionID", session.ID),
			zap.Int("bytes", n))

		msg := ForwardMessage{
			Type:      "data",
			SessionID: session.ID,
			Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		if err := c.sendForwardMessage(msg); err != nil {
			c.config.Logger.Error("Failed to forward data to server", zap.Error(err))
			break
		}
	}
}
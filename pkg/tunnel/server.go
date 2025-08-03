package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Server struct {
	logger         *zap.Logger
	authToken      string
	clients        map[string]*ServerClient
	mu             sync.RWMutex
	upgrader       websocket.Upgrader
	forwardHandler *ForwardHandler
}

type ServerClient struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte
	Server   *Server
	LastPing time.Time
}

type Message struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Target  string          `json:"target,omitempty"`
	Port    int             `json:"port,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewServer(logger *zap.Logger, authToken string) *Server {
	return &Server{
		logger:         logger,
		authToken:      authToken,
		clients:        make(map[string]*ServerClient),
		forwardHandler: NewForwardHandler(logger),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // In production, implement proper origin checking
			},
		},
	}
}

func (s *Server) HandleTunnel(w http.ResponseWriter, r *http.Request) {
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

	client := &ServerClient{
		ID:       clientID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Server:   s,
		LastPing: time.Now(),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.logger.Info("Client connected", zap.String("clientID", clientID))

	go client.writePump()
	go client.readPump()
}

func (c *ServerClient) readPump() {
	defer func() {
		c.Server.mu.Lock()
		delete(c.Server.clients, c.ID)
		c.Server.mu.Unlock()
		c.Conn.Close()
		c.Server.logger.Info("Client disconnected", zap.String("clientID", c.ID))
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.LastPing = time.Now()
		return nil
	})

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Server.logger.Error("WebSocket error", zap.Error(err))
			}
			break
		}

		c.Server.handleMessage(c, &msg)
	}
}

func (c *ServerClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.Conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleMessage(client *ServerClient, msg *Message) {
	switch msg.Type {
	case "register":
		// Handle client registration
		response := Message{
			Type: "registered",
			ID:   client.ID,
		}
		data, _ := json.Marshal(response)
		client.Send <- data

	case "forward":
		// Handle forwarding request
		s.forwardHandler.HandleForward(client, msg.Data)

	case "ping":
		// Handle ping
		response := Message{Type: "pong"}
		data, _ := json.Marshal(response)
		client.Send <- data

	default:
		s.logger.Warn("Unknown message type", zap.String("type", msg.Type))
	}
}

func (s *Server) StartTCPForwarder(port int, clientID string) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		s.logger.Error("Failed to start TCP forwarder", 
			zap.Int("port", port), 
			zap.String("clientID", clientID),
			zap.Error(err))
		return
	}
	defer listener.Close()

	s.logger.Info("TCP forwarder listening", 
		zap.Int("port", port), 
		zap.String("clientID", clientID))

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.logger.Error("Accept failed", zap.Error(err))
			continue
		}

		go s.handleTCPConnection(conn, clientID, port)
	}
}

func (s *Server) handleTCPConnection(conn net.Conn, clientID string, remotePort int) {
	defer conn.Close()

	// Find the client
	s.mu.RLock()
	client, exists := s.clients[clientID]
	s.mu.RUnlock()

	if !exists {
		s.logger.Warn("Client not found for TCP connection", zap.String("clientID", clientID))
		return
	}

	// Generate session ID
	sessionID := fmt.Sprintf("%s-%d", clientID, time.Now().UnixNano())

	// Determine target based on client and port
	var target string
	switch clientID {
	case "airgap-web":
		target = "webapp:80"
	case "airgap-db":
		target = "database:5432"
	case "airgap-ssh":
		target = "ssh-server:22"
	default:
		target = fmt.Sprintf("localhost:%d", remotePort)
	}

	s.logger.Info("Starting TCP session", 
		zap.String("sessionID", sessionID),
		zap.String("clientID", clientID),
		zap.String("target", target))

	// Store session for return data first
	session := &ServerSession{
		ID:   sessionID,
		Conn: conn,
		Done: make(chan struct{}),
	}
	s.forwardHandler.AddSession(sessionID, session)
	defer func() {
		close(session.Done)
		s.forwardHandler.RemoveSession(sessionID)
		
		// Send disconnect to client
		disconnectMsg := ForwardMessage{
			Type:      "disconnect",
			SessionID: sessionID,
		}
		s.sendForwardMessageToClient(client, disconnectMsg)
	}()

	// Send connect message to client
	connectMsg := ForwardMessage{
		Type:      "connect",
		SessionID: sessionID,
		Target:    target,
	}

	if err := s.sendForwardMessageToClient(client, connectMsg); err != nil {
		s.logger.Error("Failed to send connect message", zap.Error(err))
		return
	}

	s.logger.Info("Sent connect message to client", zap.String("sessionID", sessionID))

	// Start goroutine to forward data from external connection to client
	go func() {
		defer func() {
			s.logger.Debug("External to client forwarder stopped", zap.String("sessionID", sessionID))
			// Signal session to close when external connection closes
			select {
			case <-session.Done:
				// Already closed
			default:
				close(session.Done)
			}
		}()

		buffer := make([]byte, 32*1024)
		for {
			select {
			case <-session.Done:
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					s.logger.Debug("External connection read error", zap.String("sessionID", sessionID), zap.Error(err))
				}
				return
			}

			dataMsg := ForwardMessage{
				Type:      "data",
				SessionID: sessionID,
				Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
			}

			if err := s.sendForwardMessageToClient(client, dataMsg); err != nil {
				s.logger.Error("Failed to send data to client", zap.Error(err))
				return
			}
		}
	}()

	// Keep the main handler alive until session ends
	s.logger.Info("Waiting for session to complete", zap.String("sessionID", sessionID))
	<-session.Done
	s.logger.Info("Session completed", zap.String("sessionID", sessionID))
}

func (s *Server) sendForwardMessageToClient(client *ServerClient, msg ForwardMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	wrappedMsg := Message{
		Type: "forward",
		Data: data,
	}

	msgData, err := json.Marshal(wrappedMsg)
	if err != nil {
		return err
	}

	select {
	case client.Send <- msgData:
		return nil
	default:
		return fmt.Errorf("client send buffer full")
	}
}
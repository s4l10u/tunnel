package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ForwardMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Target    string `json:"target"`
	Port      int    `json:"port"`
	Data      string `json:"data,omitempty"` // base64 encoded
	Error     string `json:"error,omitempty"`
}

type Session struct {
	ID         string
	LocalConn  net.Conn
	RemoteAddr string
	Client     *Client
}

// StartForwarder starts listening on a local port and forwards to remote via tunnel
func (c *Client) StartForwarder(localPort int, remoteHost string, remotePort int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", localPort, err)
	}

	c.config.Logger.Info("Started forwarder",
		zap.Int("localPort", localPort),
		zap.String("remoteHost", remoteHost),
		zap.Int("remotePort", remotePort))

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				c.config.Logger.Error("Accept failed", zap.Error(err))
				continue
			}

			sessionID := fmt.Sprintf("%s-%d", c.config.ClientID, time.Now().UnixNano())
			session := &Session{
				ID:         sessionID,
				LocalConn:  conn,
				RemoteAddr: fmt.Sprintf("%s:%d", remoteHost, remotePort),
				Client:     c,
			}

			go session.handle()
		}
	}()

	return nil
}

func (s *Session) handle() {
	defer s.LocalConn.Close()

	// Send connect message
	msg := ForwardMessage{
		Type:      "connect",
		SessionID: s.ID,
		Target:    s.RemoteAddr,
	}
	
	if err := s.Client.sendForwardMessage(msg); err != nil {
		s.Client.config.Logger.Error("Failed to send connect message", zap.Error(err))
		return
	}

	// Start forwarding data
	buffer := make([]byte, 32*1024)
	for {
		n, err := s.LocalConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				s.Client.config.Logger.Error("Read error", zap.Error(err))
			}
			break
		}

		msg := ForwardMessage{
			Type:      "data",
			SessionID: s.ID,
			Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		if err := s.Client.sendForwardMessage(msg); err != nil {
			s.Client.config.Logger.Error("Failed to send data", zap.Error(err))
			break
		}
	}

	// Send disconnect message
	msg = ForwardMessage{
		Type:      "disconnect",
		SessionID: s.ID,
	}
	s.Client.sendForwardMessage(msg)
}

func (c *Client) sendForwardMessage(msg ForwardMessage) error {
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

// Server-side forwarding handler
type ForwardHandler struct {
	sessions       map[string]*RemoteSession
	serverSessions map[string]*ServerSession
	mu             sync.RWMutex
	logger         *zap.Logger
}

type ServerSession struct {
	ID   string
	Conn net.Conn
	Done chan struct{}
}

type RemoteSession struct {
	ID        string
	Conn      net.Conn
	Client    *ServerClient
	Target    string
}

func NewForwardHandler(logger *zap.Logger) *ForwardHandler {
	return &ForwardHandler{
		sessions:       make(map[string]*RemoteSession),
		serverSessions: make(map[string]*ServerSession),
		logger:         logger,
	}
}

func (h *ForwardHandler) AddSession(sessionID string, session *ServerSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.serverSessions[sessionID] = session
}

func (h *ForwardHandler) RemoveSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.serverSessions, sessionID)
}

func (h *ForwardHandler) HandleForward(client *ServerClient, data json.RawMessage) {
	var msg ForwardMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.logger.Error("Failed to unmarshal forward message", zap.Error(err))
		return
	}

	switch msg.Type {
	case "connect":
		h.handleConnect(client, &msg)
	case "connected":
		h.handleConnected(client, &msg)
	case "data":
		h.handleDataFromClient(client, &msg)
	case "disconnect":
		h.handleDisconnectFromClient(client, &msg)
	case "error":
		h.handleErrorFromClient(client, &msg)
	}
}

func (h *ForwardHandler) handleConnect(client *ServerClient, msg *ForwardMessage) {
	conn, err := net.Dial("tcp", msg.Target)
	if err != nil {
		h.logger.Error("Failed to connect to target", 
			zap.String("target", msg.Target),
			zap.Error(err))
		
		errMsg := ForwardMessage{
			Type:      "error",
			SessionID: msg.SessionID,
			Error:     err.Error(),
		}
		h.sendToClient(client, errMsg)
		return
	}

	session := &RemoteSession{
		ID:     msg.SessionID,
		Conn:   conn,
		Client: client,
		Target: msg.Target,
	}

	h.mu.Lock()
	h.sessions[msg.SessionID] = session
	h.mu.Unlock()

	// Start reading from remote
	go h.readFromRemote(session)

	h.logger.Info("Established remote connection",
		zap.String("sessionID", msg.SessionID),
		zap.String("target", msg.Target))
}

func (h *ForwardHandler) handleDataFromClient(client *ServerClient, msg *ForwardMessage) {
	// This handles data coming FROM the air-gapped client TO external users
	h.mu.RLock()
	serverSession, exists := h.serverSessions[msg.SessionID]
	h.mu.RUnlock()

	if !exists {
		h.logger.Warn("Server session not found", zap.String("sessionID", msg.SessionID))
		return
	}

	data, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		h.logger.Error("Failed to decode data", zap.Error(err))
		return
	}

	if _, err := serverSession.Conn.Write(data); err != nil {
		h.logger.Error("Failed to write to external connection", zap.Error(err))
		h.handleDisconnectFromClient(client, msg)
	}
}

func (h *ForwardHandler) handleDisconnectFromClient(client *ServerClient, msg *ForwardMessage) {
	// This handles disconnect coming FROM the air-gapped client
	h.logger.Info("Handling disconnect from client", zap.String("sessionID", msg.SessionID))
	
	h.mu.Lock()
	serverSession, exists := h.serverSessions[msg.SessionID]
	if exists {
		delete(h.serverSessions, msg.SessionID)
	}
	h.mu.Unlock()

	if exists {
		if serverSession.Conn != nil {
			serverSession.Conn.Close()
		}
		// Signal the session to end
		select {
		case <-serverSession.Done:
			// Already closed
		default:
			close(serverSession.Done)
		}
		h.logger.Info("Closed external connection", zap.String("sessionID", msg.SessionID))
	} else {
		h.logger.Warn("Session not found for disconnect", zap.String("sessionID", msg.SessionID))
	}
}

func (h *ForwardHandler) readFromRemote(session *RemoteSession) {
	defer func() {
		h.mu.Lock()
		delete(h.sessions, session.ID)
		h.mu.Unlock()
		session.Conn.Close()
	}()

	buffer := make([]byte, 32*1024)
	for {
		n, err := session.Conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				h.logger.Error("Remote read error", zap.Error(err))
			}
			break
		}

		msg := ForwardMessage{
			Type:      "data",
			SessionID: session.ID,
			Data:      base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		h.sendToClient(session.Client, msg)
	}

	// Send disconnect
	msg := ForwardMessage{
		Type:      "disconnect",
		SessionID: session.ID,
	}
	h.sendToClient(session.Client, msg)
}

func (h *ForwardHandler) handleConnected(client *ServerClient, msg *ForwardMessage) {
	h.logger.Info("Client connected to local service", 
		zap.String("sessionID", msg.SessionID))
}

func (h *ForwardHandler) handleErrorFromClient(client *ServerClient, msg *ForwardMessage) {
	h.logger.Error("Client connection error", 
		zap.String("sessionID", msg.SessionID),
		zap.String("error", msg.Error))
	
	// Close the server session
	h.mu.Lock()
	serverSession, exists := h.serverSessions[msg.SessionID]
	if exists {
		delete(h.serverSessions, msg.SessionID)
		serverSession.Conn.Close()
	}
	h.mu.Unlock()
}

func (h *ForwardHandler) sendToClient(client *ServerClient, msg ForwardMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal message", zap.Error(err))
		return
	}

	wrappedMsg := Message{
		Type: "forward",
		Data: data,
	}

	msgData, _ := json.Marshal(wrappedMsg)
	select {
	case client.Send <- msgData:
	default:
		h.logger.Warn("Client send buffer full")
	}
}
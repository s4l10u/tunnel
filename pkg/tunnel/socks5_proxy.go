package tunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"go.uber.org/zap"
)

// SOCKS5Proxy provides a SOCKS5 proxy that routes through the tunnel
type SOCKS5Proxy struct {
	logger       *zap.Logger
	tunnelClient *ImprovedClient
	listener     net.Listener
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewSOCKS5Proxy(logger *zap.Logger, tunnelClient *ImprovedClient) *SOCKS5Proxy {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &SOCKS5Proxy{
		logger:       logger,
		tunnelClient: tunnelClient,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (s *SOCKS5Proxy) Start(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 listener: %w", err)
	}
	
	s.listener = listener
	s.logger.Info("SOCKS5 proxy started", zap.Int("port", port))
	
	go s.acceptConnections()
	return nil
}

func (s *SOCKS5Proxy) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				s.logger.Error("SOCKS5 accept failed", zap.Error(err))
				continue
			}
			
			go s.handleSOCKS5Connection(conn)
		}
	}
}

func (s *SOCKS5Proxy) handleSOCKS5Connection(conn net.Conn) {
	defer conn.Close()
	
	// SOCKS5 handshake
	if !s.performHandshake(conn) {
		return
	}
	
	// Get connection request
	targetHost, targetPort, err := s.parseConnectionRequest(conn)
	if err != nil {
		s.logger.Error("Failed to parse SOCKS5 request", zap.Error(err))
		return
	}
	
	// Connect through tunnel
	target := fmt.Sprintf("%s:%d", targetHost, targetPort)
	tunnelConn, err := s.connectThroughTunnel(target)
	if err != nil {
		s.logger.Error("Failed to connect through tunnel", zap.Error(err))
		s.sendConnectionResponse(conn, 0x05) // Connection refused
		return
	}
	defer tunnelConn.Close()
	
	// Send success response
	s.sendConnectionResponse(conn, 0x00) // Success
	
	// Start forwarding
	go io.Copy(tunnelConn, conn)
	io.Copy(conn, tunnelConn)
}

func (s *SOCKS5Proxy) performHandshake(conn net.Conn) bool {
	// Read version and method count
	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return false
	}
	
	version := buffer[0]
	methodCount := buffer[1]
	
	if version != 0x05 {
		return false
	}
	
	// Read methods
	methods := make([]byte, methodCount)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return false
	}
	
	// Respond with no authentication required
	response := []byte{0x05, 0x00}
	conn.Write(response)
	
	return true
}

func (s *SOCKS5Proxy) parseConnectionRequest(conn net.Conn) (string, int, error) {
	// Read request header
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return "", 0, err
	}
	
	version := buffer[0]
	command := buffer[1]
	addressType := buffer[3]
	
	if version != 0x05 || command != 0x01 { // CONNECT command
		return "", 0, fmt.Errorf("invalid SOCKS5 request")
	}
	
	var host string
	var port int
	
	switch addressType {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		host = fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])
		
	case 0x03: // Domain name
		lengthByte := make([]byte, 1)
		if _, err := io.ReadFull(conn, lengthByte); err != nil {
			return "", 0, err
		}
		
		domainLength := lengthByte[0]
		domain := make([]byte, domainLength)
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", 0, err
		}
		host = string(domain)
		
	default:
		return "", 0, fmt.Errorf("unsupported address type: %d", addressType)
	}
	
	// Read port
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", 0, err
	}
	port = int(binary.BigEndian.Uint16(portBytes))
	
	return host, port, nil
}

func (s *SOCKS5Proxy) sendConnectionResponse(conn net.Conn, status byte) {
	response := []byte{
		0x05,       // Version
		status,     // Status
		0x00,       // Reserved
		0x01,       // Address type (IPv4)
		0, 0, 0, 0, // Bind address (0.0.0.0)
		0, 0,       // Bind port (0)
	}
	conn.Write(response)
}

func (s *SOCKS5Proxy) connectThroughTunnel(target string) (net.Conn, error) {
	// This would integrate with the tunnel client
	// For now, direct connection (would be replaced with tunnel logic)
	return net.Dial("tcp", target)
}

func (s *SOCKS5Proxy) Stop() {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
}
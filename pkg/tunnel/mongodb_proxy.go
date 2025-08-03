package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MongoDBProxy provides transparent TCP tunneling for MongoDB connections
type MongoDBProxy struct {
	logger          *zap.Logger
	tunnelClient    *ImprovedClient
	localListener   net.Listener
	mongoTarget     string   // Single MongoDB target for transparent proxy
	ctx            context.Context
	cancel         context.CancelFunc
}

// Removed MongoDBMessage struct as transparent proxy doesn't need protocol parsing

func NewMongoDBProxy(logger *zap.Logger, tunnelClient *ImprovedClient, localPort int, mongoTargets []string, replicaSetName string) *MongoDBProxy {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Use first target for transparent proxy
	var target string
	if len(mongoTargets) > 0 {
		target = mongoTargets[0]
	}
	
	return &MongoDBProxy{
		logger:         logger,
		tunnelClient:   tunnelClient,
		mongoTarget:    target,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetTunnelHost is kept for compatibility but not used in transparent mode
func (p *MongoDBProxy) SetTunnelHost(host string) {
	// No-op in transparent proxy mode
}

func (p *MongoDBProxy) Start(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to create MongoDB proxy listener: %w", err)
	}
	
	p.localListener = listener
	p.logger.Info("MongoDB proxy started", zap.Int("port", port))
	
	go p.acceptConnections()
	return nil
}

func (p *MongoDBProxy) acceptConnections() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			conn, err := p.localListener.Accept()
			if err != nil {
				p.logger.Error("Accept failed", zap.Error(err))
				continue
			}
			
			go p.handleConnection(conn)
		}
	}
}

func (p *MongoDBProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	
	// Request tunnel connection to MongoDB target
	mongoConn, err := p.connectToMongoDB(p.mongoTarget)
	if err != nil {
		p.logger.Error("Failed to connect to MongoDB through tunnel", 
			zap.String("target", p.mongoTarget),
			zap.Error(err))
		return
	}
	defer mongoConn.Close()
	
	p.logger.Info("Established tunnel connection", 
		zap.String("target", p.mongoTarget))
	
	// Start transparent bidirectional forwarding
	var wg sync.WaitGroup
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		p.forwardTraffic(clientConn, mongoConn, "client->mongo")
	}()
	
	go func() {
		defer wg.Done()
		p.forwardTraffic(mongoConn, clientConn, "mongo->client")
	}()
	
	wg.Wait()
}

func (p *MongoDBProxy) connectToMongoDB(target string) (net.Conn, error) {
	if p.tunnelClient == nil {
		return nil, fmt.Errorf("tunnel client not available")
	}
	
	// Create a new tunnel session for the MongoDB connection
	// This will be handled by the tunnel client's session management
	sessionConn, err := p.tunnelClient.CreateTunnelSession(target)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel session: %w", err)
	}
	
	return sessionConn, nil
}

// forwardTraffic provides transparent TCP forwarding without protocol parsing
func (p *MongoDBProxy) forwardTraffic(src, dst net.Conn, direction string) {
	buffer := make([]byte, 32*1024) // 32KB buffer
	
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			// Set read timeout to allow context cancellation
			src.SetReadDeadline(time.Now().Add(30 * time.Second))
			
			n, err := src.Read(buffer)
			if err != nil {
				if err != io.EOF {
					p.logger.Debug("Read failed", 
						zap.Error(err), 
						zap.String("direction", direction))
				}
				return
			}
			
			if n > 0 {
				_, err = dst.Write(buffer[:n])
				if err != nil {
					p.logger.Debug("Write failed", 
						zap.Error(err), 
						zap.String("direction", direction))
					return
				}
				
				p.logger.Debug("Forwarded data", 
					zap.Int("bytes", n), 
					zap.String("direction", direction))
			}
		}
	}
}

// Removed MongoDB-specific message processing for transparent proxy mode
// All MongoDB protocol handling is done transparently without modification

func (p *MongoDBProxy) Stop() {
	p.cancel()
	if p.localListener != nil {
		p.localListener.Close()
	}
}
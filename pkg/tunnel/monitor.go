package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// HealthStatus represents the health status of a tunnel component
type HealthStatus struct {
	Status         string                 `json:"status"`
	Implementation string                 `json:"implementation"`
	Uptime         string                 `json:"uptime"`
	Metrics        map[string]interface{} `json:"metrics"`
	Clients        []ClientHealth         `json:"clients,omitempty"`
	Sessions       []SessionHealth        `json:"sessions,omitempty"`
	Errors         []ErrorInfo            `json:"errors,omitempty"`
}

// ClientHealth represents health information for a connected client
type ClientHealth struct {
	ID             string    `json:"id"`
	Connected      bool      `json:"connected"`
	LastPing       time.Time `json:"lastPing"`
	ActiveSessions int       `json:"activeSessions"`
}

// SessionHealth represents health information for an active session
type SessionHealth struct {
	ID            string    `json:"id"`
	ClientID      string    `json:"clientId"`
	Target        string    `json:"target"`
	CreatedAt     time.Time `json:"createdAt"`
	BytesIn       int64     `json:"bytesIn"`
	BytesOut      int64     `json:"bytesOut"`
	LastActivity  time.Time `json:"lastActivity"`
}

// ErrorInfo represents recent error information
type ErrorInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Count     int       `json:"count"`
}

// Monitor provides health monitoring for tunnel components
type Monitor struct {
	startTime      time.Time
	errorBuffer    *CircularBuffer
	metricsStore   *MetricsStore
	logger         *zap.Logger
	implementation string
}

// NewMonitor creates a new monitor instance
func NewMonitor(implementation string, logger *zap.Logger) *Monitor {
	return &Monitor{
		startTime:      time.Now(),
		errorBuffer:    NewCircularBuffer(100),
		metricsStore:   NewMetricsStore(),
		logger:         logger,
		implementation: implementation,
	}
}

// CircularBuffer holds a fixed number of recent errors
type CircularBuffer struct {
	buffer []ErrorInfo
	size   int
	head   int
	mu     sync.RWMutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]ErrorInfo, size),
		size:   size,
	}
}

func (cb *CircularBuffer) Add(err ErrorInfo) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.buffer[cb.head] = err
	cb.head = (cb.head + 1) % cb.size
}

func (cb *CircularBuffer) GetRecent(count int) []ErrorInfo {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if count > cb.size {
		count = cb.size
	}
	
	result := make([]ErrorInfo, 0, count)
	for i := 0; i < count; i++ {
		idx := (cb.head - 1 - i + cb.size) % cb.size
		if cb.buffer[idx].Timestamp.IsZero() {
			break
		}
		result = append(result, cb.buffer[idx])
	}
	
	return result
}

// MetricsStore holds various metrics
type MetricsStore struct {
	connectionsTotal    atomic.Int64
	connectionsActive   atomic.Int32
	messagesTotal       atomic.Int64
	bytesTransferred    atomic.Int64
	errorsTotal         atomic.Int64
	reconnectsTotal     atomic.Int64
	sessionsTotal       atomic.Int64
	sessionsActive      atomic.Int32
	lastError           atomic.Value
	lastErrorTime       atomic.Value
}

func NewMetricsStore() *MetricsStore {
	return &MetricsStore{}
}

func (ms *MetricsStore) IncrementConnections() {
	ms.connectionsTotal.Add(1)
	ms.connectionsActive.Add(1)
}

func (ms *MetricsStore) DecrementConnections() {
	ms.connectionsActive.Add(-1)
}

func (ms *MetricsStore) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"connectionsTotal":  ms.connectionsTotal.Load(),
		"connectionsActive": ms.connectionsActive.Load(),
		"messagesTotal":     ms.messagesTotal.Load(),
		"bytesTransferred":  ms.bytesTransferred.Load(),
		"errorsTotal":       ms.errorsTotal.Load(),
		"reconnectsTotal":   ms.reconnectsTotal.Load(),
		"sessionsTotal":     ms.sessionsTotal.Load(),
		"sessionsActive":    ms.sessionsActive.Load(),
	}
}

// RecordError records an error occurrence
func (m *Monitor) RecordError(level, message string) {
	m.metricsStore.errorsTotal.Add(1)
	m.metricsStore.lastError.Store(message)
	m.metricsStore.lastErrorTime.Store(time.Now())
	
	m.errorBuffer.Add(ErrorInfo{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Count:     1,
	})
}

// GetHealth returns current health status
func (m *Monitor) GetHealth() HealthStatus {
	uptime := time.Since(m.startTime)
	
	status := HealthStatus{
		Status:         "healthy",
		Implementation: m.implementation,
		Uptime:         fmt.Sprintf("%v", uptime.Round(time.Second)),
		Metrics:        m.metricsStore.GetMetrics(),
		Errors:         m.errorBuffer.GetRecent(10),
	}
	
	// Determine overall status based on metrics
	activeConns := m.metricsStore.connectionsActive.Load()
	errorRate := float64(m.metricsStore.errorsTotal.Load()) / float64(time.Since(m.startTime).Seconds())
	
	if activeConns == 0 {
		status.Status = "degraded"
	} else if errorRate > 1.0 { // More than 1 error per second
		status.Status = "unhealthy"
	}
	
	return status
}

// HTTPHealthHandler returns an HTTP handler for health checks
func (m *Monitor) HTTPHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := m.GetHealth()
		
		w.Header().Set("Content-Type", "application/json")
		
		statusCode := http.StatusOK
		switch health.Status {
		case "degraded":
			statusCode = http.StatusServiceUnavailable
		case "unhealthy":
			statusCode = http.StatusServiceUnavailable
		}
		
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(health)
	}
}

// ImprovedServerMonitor extends Monitor for the improved server
type ImprovedServerMonitor struct {
	*Monitor
	server *ImprovedServer
}

func NewImprovedServerMonitor(server *ImprovedServer) *ImprovedServerMonitor {
	return &ImprovedServerMonitor{
		Monitor: NewMonitor("improved-server", server.logger),
		server:  server,
	}
}

// GetHealth returns detailed health status for improved server
func (ism *ImprovedServerMonitor) GetHealth() HealthStatus {
	health := ism.Monitor.GetHealth()
	
	// Add client information
	ism.server.clients.mu.RLock()
	defer ism.server.clients.mu.RUnlock()
	
	for _, client := range ism.server.clients.clients {
		clientHealth := ClientHealth{
			ID:        client.ID,
			Connected: true,
			LastPing:  time.Unix(client.lastPing.Load(), 0),
		}
		
		// Count active sessions for this client
		activeSessions := 0
		ism.server.sessions.mu.RLock()
		for _, session := range ism.server.sessions.sessions {
			if session.ClientID == client.ID && !session.closed.Load() {
				activeSessions++
			}
		}
		ism.server.sessions.mu.RUnlock()
		
		clientHealth.ActiveSessions = activeSessions
		health.Clients = append(health.Clients, clientHealth)
	}
	
	// Add session information (limited to recent 10)
	ism.server.sessions.mu.RLock()
	defer ism.server.sessions.mu.RUnlock()
	
	count := 0
	for _, session := range ism.server.sessions.sessions {
		if count >= 10 {
			break
		}
		
		if !session.closed.Load() {
			sessionHealth := SessionHealth{
				ID:        session.ID,
				ClientID:  session.ClientID,
				Target:    session.Target,
				CreatedAt: time.Now(), // Would need to track this
			}
			health.Sessions = append(health.Sessions, sessionHealth)
			count++
		}
	}
	
	return health
}

// CircuitBreaker implements circuit breaker pattern for connection reliability
type CircuitBreaker struct {
	name          string
	maxFailures   int
	resetTimeout  time.Duration
	failures      atomic.Int32
	lastFailure   atomic.Value
	state         atomic.Value // "closed", "open", "half-open"
	successCount  atomic.Int32
	logger        *zap.Logger
}

func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration, logger *zap.Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		logger:       logger,
	}
	cb.state.Store("closed")
	return cb
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	state := cb.state.Load().(string)
	
	switch state {
	case "open":
		lastFailureTime, ok := cb.lastFailure.Load().(time.Time)
		if !ok || time.Since(lastFailureTime) > cb.resetTimeout {
			cb.state.Store("half-open")
			cb.logger.Info("Circuit breaker moving to half-open", zap.String("name", cb.name))
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
		
	case "half-open":
		// Allow one request through
		if cb.successCount.Load() > 0 {
			cb.state.Store("closed")
			cb.failures.Store(0)
			cb.successCount.Store(0)
			cb.logger.Info("Circuit breaker closed", zap.String("name", cb.name))
		}
	}
	
	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}
	
	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) recordFailure() {
	failures := cb.failures.Add(1)
	cb.lastFailure.Store(time.Now())
	
	if failures >= int32(cb.maxFailures) {
		cb.state.Store("open")
		cb.logger.Warn("Circuit breaker opened", 
			zap.String("name", cb.name),
			zap.Int32("failures", failures))
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	state := cb.state.Load().(string)
	if state == "half-open" {
		cb.successCount.Add(1)
	}
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:   5,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
	}
}

// ExecuteWithRetry executes a function with retry logic
func ExecuteWithRetry(ctx context.Context, policy RetryPolicy, fn func() error, logger *zap.Logger) error {
	var lastErr error
	delay := policy.InitialDelay
	
	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if attempt > 0 {
			logger.Debug("Retrying operation", 
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay))
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		
		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * policy.BackoffFactor)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
		
		// Add jitter
		jitter := time.Duration(float64(delay) * policy.JitterFactor * (2*rand.Float64() - 1))
		delay += jitter
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", policy.MaxAttempts, lastErr)
}
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
)

// Logger provides unified logging across all extensions
type Logger struct {
	impl    log.Logger
	mu      sync.Mutex
	prefix  string
	level   string
}

// NewLogger creates a new logger instance
func NewLogger(impl log.Logger) *Logger {
	return &Logger{
		impl:  impl,
		level: "info",
	}
}

// Info logs info level messages
func (l *Logger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	l.impl.Info(msg)
}

// Debug logs debug level messages
func (l *Logger) Debug(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level == "debug" {
		if l.prefix != "" {
			msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
		}
		l.impl.Debug(msg)
	}
}

// Warn logs warning level messages
func (l *Logger) Warn(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	l.impl.Warn(msg)
}

// Error logs error level messages
func (l *Logger) Error(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.prefix != "" {
		msg = fmt.Sprintf("[%s] %s", l.prefix, msg)
	}
	l.impl.Error(msg)
}

// SetPrefix sets logger prefix
func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

// SetLevel sets log level
func (l *Logger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// TLSConfig provides TLS configuration utilities
type TLSConfig struct {
	ServerName string
	CA         string
	Cert       string
	Key        string
	Insecure   bool
	NextProto  string
}

// BuildTLSConfig creates TLS config from parameters
func BuildTLSConfig(serverName, ca, cert, key string, insecure bool) (*TLSConfig, error) {
	return &TLSConfig{
		ServerName: serverName,
		CA:         ca,
		Cert:       cert,
		Key:        key,
		Insecure:   insecure,
	}, nil
}

// Connection provides connection management utilities
type Connection struct {
	net.Conn
	lastActivity time.Time
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		Conn:         conn,
		lastActivity: time.Now(),
	}
}

// Read reads data and updates last activity
func (c *Connection) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err == nil && n > 0 {
		c.lastActivity = time.Now()
	}
	return
}

// Write writes data and updates last activity
func (c *Connection) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if err == nil && n > 0 {
		c.lastActivity = time.Now()
	}
	return
}

// LastActivity returns last activity time
func (c *Connection) LastActivity() time.Time {
	return c.lastActivity
}

// IsIdle checks if connection has been idle for duration
func (c *Connection) IsIdle(duration time.Duration) bool {
	return time.Since(c.lastActivity) > duration
}

// Encryption provides encryption utilities
type Encryption struct{}

// GenerateKey generates a random key
func (e *Encryption) GenerateKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// GenerateBase64Key generates a base64 encoded random key
func (e *Encryption) GenerateBase64Key(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Encode encodes data to base64
func (e *Encryption) Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Decode decodes data from base64
func (e *Encryption) Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// Network provides network utilities
type Network struct{}

// ParseAddress parses address string to host and port
func (n *Network) ParseAddress(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	
	// If no port specified, assume common ports
	if port == "" {
		if strings.Contains(address, ":") {
			port = "443"
		} else {
			port = "80"
		}
	}
	
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return "", 0, err
	}
	
	return host, portNum, nil
}

// IsLocal checks if address is local
func (n *Network) IsLocal(address string) bool {
	host, _, err := n.ParseAddress(address)
	if err != nil {
		return false
	}
	
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	
	// Check if it's localhost
	if ip.IsLoopback() {
		return true
	}
	
	// Check private IP ranges
	if ip.IsPrivate() {
		return true
	}
	
	return false
}

// ResolveHost resolves hostname to IP
func (n *Network) ResolveHost(host string) (string, error) {
	// If already an IP, return it
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	
	// Resolve hostname
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	
	// Return first IPv4 address if available, otherwise first address
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	
	if len(ips) > 0 {
		return ips[0].String(), nil
	}
	
	return "", fmt.Errorf("no IP found for host: %s", host)
}

// Retry provides retry utilities
type Retry struct {
	MaxAttempts int
	Delay       time.Duration
	Backoff     float64
}

// NewRetry creates a new retry configuration
func NewRetry(maxAttempts int, delay time.Duration) *Retry {
	return &Retry{
		MaxAttempts: maxAttempts,
		Delay:       delay,
		Backoff:     1.5,
	}
}

// Do performs operation with retry
func (r *Retry) Do(operation func() error) error {
	var lastErr error
	delay := r.Delay
	
	for attempt := 1; attempt <= r.MaxAttempts; attempt++ {
		lastErr = operation()
		if lastErr == nil {
			return nil
		}
		
		if attempt < r.MaxAttempts {
			log.Printf("Attempt %d failed: %v, retrying in %v", attempt, lastErr, delay)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * r.Backoff)
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %v", r.MaxAttempts, lastErr)
}

// Metrics provides metrics collection
type Metrics struct {
	mu           sync.RWMutex
	bytesSent    uint64
	bytesReceived uint64
	connections  int
	startTime    time.Time
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// AddBytesSent adds to bytes sent counter
func (m *Metrics) AddBytesSent(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytesSent += uint64(n)
}

// AddBytesReceived adds to bytes received counter
func (m *Metrics) AddBytesReceived(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bytesReceived += uint64(n)
}

// IncConnections increments connection counter
func (m *Metrics) IncConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections++
}

// DecConnections decrements connection counter
func (m *Metrics) DecConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connections > 0 {
		m.connections--
	}
}

// GetStats returns current metrics
func (m *Metrics) GetStats() struct {
	BytesSent     uint64
	BytesReceived uint64
	Connections   int
	Uptime        time.Duration
} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return struct {
		BytesSent     uint64
		BytesReceived uint64
		Connections   int
		Uptime        time.Duration
	}{
		BytesSent:     m.bytesSent,
		BytesReceived: m.bytesReceived,
		Connections:   m.connections,
		Uptime:        time.Since(m.startTime),
	}
}

// Config provides configuration utilities
type Config struct{}

// ValidateJSON validates JSON configuration
func (c *Config) ValidateJSON(jsonData []byte) error {
	var data interface{}
	return c.UnmarshalJSON(jsonData, &data)
}

// UnmarshalJSON unmarshals JSON with error handling
func (c *Config) UnmarshalJSON(jsonData []byte, target interface{}) error {
	if len(jsonData) == 0 {
		return fmt.Errorf("empty configuration")
	}
	
	// Basic validation - check if it's valid JSON
	var tmp map[string]interface{}
	if err := json.Unmarshal(jsonData, &tmp); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	
	// Unmarshal to target
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %v", err)
	}
	
	return nil
}

// MarshalJSON marshals configuration with formatting
func (c *Config) MarshalJSON(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// Timeouts provides timeout utilities
type Timeouts struct {
	Connect time.Duration
	Read    time.Duration
	Write   time.Duration
	Idle    time.Duration
}

// NewTimeouts creates default timeouts
func NewTimeouts() *Timeouts {
	return &Timeouts{
		Connect: 30 * time.Second,
		Read:    60 * time.Second,
		Write:   60 * time.Second,
		Idle:    300 * time.Second,
	}
}

// ApplyToConn applies timeouts to connection
func (t *Timeouts) ApplyToConn(conn net.Conn) error {
	if t.Connect > 0 {
		if err := conn.SetDeadline(time.Now().Add(t.Connect)); err != nil {
			return err
		}
	}
	return nil
}

// ApplyReadTimeout applies read timeout to connection
func (t *Timeouts) ApplyReadTimeout(conn net.Conn) error {
	if t.Read > 0 {
		return conn.SetReadDeadline(time.Now().Add(t.Read))
	}
	return nil
}

// ApplyWriteTimeout applies write timeout to connection
func (t *Timeouts) ApplyWriteTimeout(conn net.Conn) error {
	if t.Write > 0 {
		return conn.SetWriteDeadline(time.Now().Add(t.Write))
	}
	return nil
}


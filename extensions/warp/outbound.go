package warp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/sagernet/sing-box/common/register"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/quickclose"
)

func init() {
	register.Outbound("warp", NewWARPOutbound)
	register.Outbound("warp-plus", NewWARPPlusOutbound)
	register.Outbound("warp-tls", NewWARPTLSOutbound)
}

type WARPConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Mode           string `json:"mode"` // "wireguard", "warp+", "team"
	PrivateKey     string `json:"private_key"`
	PublicKey      string `json:"public_key"`
	Endpoint       string `json:"endpoint"` // WARP endpoint
	Reserved       string `json:"reserved"` // Base64 encoded reserved bytes
	PSK            string `json:"psk"`      // Pre-shared key
	LicenseKey     string `json:"license_key"` // For WARP+
	TeamID         string `json:"team_id"` // For team mode
	BindInterface  string `json:"bind_interface"` // Network interface to bind
	DNS            string `json:"dns"` // DNS server for WARP
	MTU            int    `json:"mtu"` // MTU for WireGuard tunnel
	TLSConfig      TLSConfig `json:"tls_config"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type WARPBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     WARPConfig
	connection net.Conn
	wireguardConn net.Conn
}

func NewWARPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*WARPOutbound, error) {
	var config WARPConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	// Set default WARP configuration
	if config.Server == "" {
		config.Server = "162.159.192.1"
	}
	if config.Port == 0 {
		config.Port = 2408
	}
	if config.Mode == "" {
		config.Mode = "wireguard"
	}

	return &WARPOutbound{
		WARPBaseOutbound: WARPBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (w *WARPBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return w.connectAndRoute(ctx, packet)
}

func (w *WARPBaseOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if w.connection == nil {
		conn, err := w.createConnection()
		if err != nil {
			return err
		}
		w.connection = conn
	}

	// Route packet through WARP tunnel
	_, err := w.connection.Write(packet.Data())
	return err
}

func (w *WARPBaseOutbound) createConnection() (net.Conn, error) {
	var conn net.Conn
	var err error

	switch w.config.Mode {
	case "wireguard":
		conn, err = w.createWireGuardConnection()
	case "warp+":
		conn, err = w.createWARPPlusConnection()
	case "team":
		conn, err = w.createTeamWARPConnection()
	default:
		conn, err = w.createWireGuardConnection()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create WARP connection: %v", err)
	}

	return conn, nil
}

func (w *WARPBaseOutbound) createWireGuardConnection() (net.Conn, error) {
	// WARP uses WireGuard protocol for UDP tunnel
	address := fmt.Sprintf("%s:%d", w.config.Server, w.config.Port)
	
	// Create UDP connection to WARP endpoint
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	// Perform WireGuard handshake
	if err := w.performWireGuardHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &WireGuardWARPConn{conn: conn, config: w.config}, nil
}

func (w *WARPBaseOutbound) createWARPPlusConnection() (net.Conn, error) {
	// WARP+ uses license key for enhanced service
	if w.config.LicenseKey == "" {
		return nil, fmt.Errorf("license key required for WARP+")
	}

	address := fmt.Sprintf("%s:%d", w.config.Server, w.config.Port)
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	if err := w.performWARPPlusHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &WARPPlusConn{conn: conn, config: w.config}, nil
}

func (w *WARPBaseOutbound) createTeamWARPConnection() (net.Conn, error) {
	// Team WARP uses team ID for enterprise features
	if w.config.TeamID == "" {
		return nil, fmt.Errorf("team ID required for team mode")
	}

	address := fmt.Sprintf("%s:%d", w.config.Server, w.config.Port)
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	if err := w.performTeamWARPHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &TeamWARPConn{conn: conn, config: w.config}, nil
}

func (w *WARPBaseOutbound) performWireGuardHandshake(conn net.Conn) error {
	w.logger.Info("Performing WireGuard handshake for WARP")
	
	// Send WireGuard handshake initiation message
	// This includes public key, timestamp, and other handshake data
	
	// For now, this is a simplified implementation
	// Real implementation would use proper WireGuard handshake protocol
	
	// Send handshake message
	handshakeMsg := w.buildWireGuardHandshake()
	_, err := conn.Write(handshakeMsg)
	return err
}

func (w *WARPBaseOutbound) performWARPPlusHandshake(conn net.Conn) error {
	w.logger.Info("Performing WARP+ handshake with license key")
	
	// WARP+ handshake includes license validation
	if w.config.LicenseKey != "" {
		// Include license key in handshake
		w.logger.Debug("Using WARP+ license key")
	}
	
	return w.performWireGuardHandshake(conn)
}

func (w *WARPBaseOutbound) performTeamWARPHandshake(conn net.Conn) error {
	w.logger.Info("Performing Team WARP handshake")
	
	// Team WARP handshake includes team authentication
	if w.config.TeamID != "" {
		// Include team ID in handshake
		w.logger.Debug("Using Team WARP ID:", w.config.TeamID)
	}
	
	return w.performWireGuardHandshake(conn)
}

func (w *WARPBaseOutbound) buildWireGuardHandshake() []byte {
	// Build WireGuard handshake initiation message
	// This is a simplified implementation
	// Real implementation would use proper WireGuard protocol
	
	// WireGuard handshake message structure:
	// Type (4 bytes) | Sender index (4 bytes) | Receiver index (4 bytes) | 
	// Ephemeral public key (32 bytes) | Timestamp (8 bytes) | MAC (16 bytes)
	
	message := make([]byte, 0, 72)
	
	// Type (handshake initiation)
	message = append(message, []byte{0x01, 0x00, 0x00, 0x00}...)
	
	// Sender index (random for first message)
	// In real implementation, this would be generated
	message = append(message, []byte{0x01, 0x02, 0x03, 0x04}...)
	
	// Receiver index (0 for first message)
	message = append(message, []byte{0x00, 0x00, 0x00, 0x00}...)
	
	// Ephemeral public key (placeholder)
	message = append(message, make([]byte, 32)...)
	
	// Timestamp (placeholder)
	message = append(message, make([]byte, 8)...)
	
	// MAC (placeholder)
	message = append(message, make([]byte, 16)...)
	
	return message
}

func (w *WARPBaseOutbound) Close() error {
	if w.connection != nil {
		return w.connection.Close()
	}
	return nil
}

func (w *WARPBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different WARP modes
type WARPOutbound struct {
	WARPBaseOutbound
}

type WARPPlusOutbound struct {
	WARPBaseOutbound
}

type WARPTLSOutbound struct {
	WARPBaseOutbound
}

// Constructor functions for each WARP mode
func NewWARPPlusOutbound(router route.Router, logger log.Logger, options option.Outbound) (*WARPPlusOutbound, error) {
	base, err := NewWARPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	// Set mode to warp+
	base.config.Mode = "warp+"
	return &WARPPlusOutbound{*base}, nil
}

func NewWARPTLSOutbound(router route.Router, logger log.Logger, options option.Outbound) (*WARPTLSOutbound, error) {
	base, err := NewWARPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	// Set mode to wireguard with TLS
	base.config.Mode = "wireguard"
	return &WARPTLSOutbound{*base}, nil
}

// Connection wrapper types
type WireGuardWARPConn struct {
	conn   net.Conn
	config WARPConfig
}

type WARPPlusConn struct {
	conn   net.Conn
	config WARPConfig
}

type TeamWARPConn struct {
	conn   net.Conn
	config WARPConfig
}

// Implement net.Conn interface
func (c *WireGuardWARPConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *WireGuardWARPConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *WireGuardWARPConn) Close() error {
	return c.conn.Close()
}

func (c *WireGuardWARPConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *WireGuardWARPConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *WireGuardWARPConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *WireGuardWARPConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WireGuardWARPConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Implement other connection wrappers similarly...
func (c *WARPPlusConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *WARPPlusConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *WARPPlusConn) Close() error {
	return c.conn.Close()
}

func (c *WARPPlusConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *WARPPlusConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *WARPPlusConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *WARPPlusConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WARPPlusConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *TeamWARPConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *TeamWARPConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *TeamWARPConn) Close() error {
	return c.conn.Close()
}

func (c *TeamWARPConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *TeamWARPConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *TeamWARPConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *TeamWARPConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *TeamWARPConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}


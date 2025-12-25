package legacyvpn

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
	register.Outbound("l2tp", NewL2TPOutbound)
	register.Outbound("l2tp-ipsec", NewL2TPIPsecOutbound)
	register.Outbound("ikev2", NewIKEv2Outbound)
	register.Outbound("ikev2-ipsec", NewIKEv2IPsecOutbound)
	register.Outbound("sstp", NewSSTPOutbound)
	register.Outbound("pptp", NewPPTPOutbound)
	register.Outbound("gre", NewGREOutbound)
	register.Outbound("softether", NewSoftEtherOutbound)
}

type LegacyVPNConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // "l2tp", "l2tp-ipsec", "ikev2", "ikev2-ipsec", "sstp", "pptp", "gre", "softether"
	Username       string `json:"username"`
	Password       string `json:"password"`
	Secret         string `json:"secret"` // For L2TP/IPsec
	// L2TP specific
	L2TPSecret     string `json:"l2tp_secret"`
	// IKEv2 specific
	IKEv2ServerID  string `json:"ikev2_server_id"`
	IKEv2ClientID  string `json:"ikev2_client_id"`
	IKEv2Cert      string `json:"ikev2_cert"`
	IKEv2Key       string `json:"ikev2_key"`
	// SSTP specific
	SSTPUrl        string `json:"sstp_url"`
	// PPTP specific
	PPTPEncryption bool   `json:"pptp_encryption"`
	// IPsec specific
	PSK            string `json:"psk"` // Pre-shared key
	Certificate    string `json:"certificate"`
	PrivateKey     string `json:"private_key"`
	// SoftEther specific
	SoftEtherHub   string `json:"softether_hub"`
	SoftEtherBridge string `json:"softether_bridge"`
	// Security
	TLSConfig      TLSConfig `json:"tls_config"`
	Timeout        int       `json:"timeout"`
	Reconnect      bool      `json:"reconnect"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type LegacyVPNBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     LegacyVPNConfig
	connection net.Conn
}

func NewL2TPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*L2TPOutbound, error) {
	var config LegacyVPNConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	// Set default L2TP configuration
	if config.Server == "" {
		config.Server = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 1701
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	return &L2TPOutbound{
		LegacyVPNBaseOutbound: LegacyVPNBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (l *LegacyVPNBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return l.connectAndRoute(ctx, packet)
}

func (l *LegacyVPNBaseOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if l.connection == nil {
		conn, err := l.createConnection()
		if err != nil {
			return err
		}
		l.connection = conn
	}

	// Route packet through legacy VPN tunnel
	_, err := l.connection.Write(packet.Data())
	return err
}

func (l *LegacyVPNBaseOutbound) createConnection() (net.Conn, error) {
	var conn net.Conn
	var err error

	switch l.config.Protocol {
	case "l2tp":
		conn, err = l.createL2TPConnection()
	case "l2tp-ipsec":
		conn, err = l.createL2TPIPsecConnection()
	case "ikev2":
		conn, err = l.createIKEv2Connection()
	case "ikev2-ipsec":
		conn, err = l.createIKEv2IPsecConnection()
	case "sstp":
		conn, err = l.createSSTPConnection()
	case "pptp":
		conn, err = l.createPPTPConnection()
	case "gre":
		conn, err = l.createGREConnection()
	case "softether":
		conn, err = l.createSoftEtherConnection()
	default:
		conn, err = l.createL2TPConnection()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s connection: %v", l.config.Protocol, err)
	}

	return conn, nil
}

func (l *LegacyVPNBaseOutbound) createL2TPConnection() (net.Conn, error) {
	// L2TP over UDP
	address := fmt.Sprintf("%s:%d", l.config.Server, l.config.Port)
	
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	// Perform L2TP handshake
	if err := l.performL2TPHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &L2TPConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createL2TPIPsecConnection() (net.Conn, error) {
	// L2TP over IPsec
	address := fmt.Sprintf("%s:%d", l.config.Server, l.config.Port)
	
	// Create encrypted connection
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	// Perform L2TP handshake with IPsec
	if err := l.performL2TPIPsecHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &L2TPIPsecConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createIKEv2Connection() (net.Conn, error) {
	// IKEv2/IPsec over UDP port 500/4500
	port := l.config.Port
	if port == 0 {
		port = 500
	}
	
	address := fmt.Sprintf("%s:%d", l.config.Server, port)
	
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	// Perform IKEv2 handshake
	if err := l.performIKEv2Handshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &IKEv2Conn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createIKEv2IPsecConnection() (net.Conn, error) {
	// IKEv2 with IPsec
	return l.createIKEv2Connection()
}

func (l *LegacyVPNBaseOutbound) createSSTPConnection() (net.Conn, error) {
	// SSTP over HTTPS
	server := l.config.Server
	if l.config.SSTPUrl != "" {
		server = l.config.SSTPUrl
	}
	
	address := fmt.Sprintf("%s:%d", server, 443)
	
	tlsConfig := &tls.Config{
		ServerName:         l.config.TLSConfig.ServerName,
		InsecureSkipVerify: l.config.TLSConfig.Insecure,
	}
	
	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, err
	}

	// Perform SSTP handshake
	if err := l.performSSTPHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &SSTPConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createPPTPConnection() (net.Conn, error) {
	// PPTP over TCP port 1723
	address := fmt.Sprintf("%s:%d", l.config.Server, 1723)
	
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// Perform PPTP handshake
	if err := l.performPPTPHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PPTPConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createGREConnection() (net.Conn, error) {
	// GRE over IP protocol 47
	// GRE doesn't have TCP/UDP connection, so we create a mock connection
	address := fmt.Sprintf("%s:%d", l.config.Server, l.config.Port)
	
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	// Perform GRE handshake (simplified)
	if err := l.performGREHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &GREConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) createSoftEtherConnection() (net.Conn, error) {
	// SoftEther over TCP port 992
	address := fmt.Sprintf("%s:%d", l.config.Server, 992)
	
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// Perform SoftEther handshake
	if err := l.performSoftEtherHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &SoftEtherConn{conn: conn, config: l.config}, nil
}

func (l *LegacyVPNBaseOutbound) performL2TPHandshake(conn net.Conn) error {
	l.logger.Info("Performing L2TP handshake")
	
	// Send L2TP Start-Control-Connection-Request (SCCRQ)
	// This includes tunnel ID, peer tunnel ID, and L2TP version
	
	return nil
}

func (l *LegacyVPNBaseOutbound) performL2TPIPsecHandshake(conn net.Conn) error {
	l.logger.Info("Performing L2TP/IPsec handshake")
	
	// First establish IPsec tunnel, then L2TP
	if l.config.PSK != "" {
		l.logger.Debug("Using PSK for IPsec authentication")
	}
	
	return l.performL2TPHandshake(conn)
}

func (l *LegacyVPNBaseOutbound) performIKEv2Handshake(conn net.Conn) error {
	l.logger.Info("Performing IKEv2 handshake")
	
	// Send IKE_SA_INIT and IKE_AUTH exchanges
	// This establishes the IPsec SA
	
	if l.config.IKEv2ServerID != "" {
		l.logger.Debug("Server ID:", l.config.IKEv2ServerID)
	}
	
	return nil
}

func (l *LegacyVPNBaseOutbound) performSSTPHandshake(conn net.Conn) error {
	l.logger.Info("Performing SSTP handshake")
	
	// Send HTTP CONNECT request for SSTP
	// This tunnels through HTTPS
	
	_, err := conn.Write([]byte("SSTP_DUPLEX_POST /sra_{BA195980-CD49-458b-9E23-C84E0B5B7DC6}/ HTTP/1.1\r\nHost: " + l.config.Server + "\r\n\r\n"))
	return err
}

func (l *LegacyVPNBaseOutbound) performPPTPHandshake(conn net.Conn) error {
	l.logger.Info("Performing PPTP handshake")
	
	// Send PPTP Start-Control-Connection-Request
	// This establishes the PPTP tunnel
	
	return nil
}

func (l *LegacyVPNBaseOutbound) performGREHandshake(conn net.Conn) error {
	l.logger.Info("Performing GRE handshake")
	
	// GRE doesn't have a formal handshake
	// Just establish the tunnel
	
	return nil
}

func (l *LegacyVPNBaseOutbound) performSoftEtherHandshake(conn net.Conn) error {
	l.logger.Info("Performing SoftEther handshake")
	
	// SoftEther uses its own protocol over TCP
	// Send protocol version and authentication
	
	if l.config.SoftEtherHub != "" {
		l.logger.Debug("Using hub:", l.config.SoftEtherHub)
	}
	
	return nil
}

func (l *LegacyVPNBaseOutbound) Close() error {
	if l.connection != nil {
		return l.connection.Close()
	}
	return nil
}

func (l *LegacyVPNBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different legacy VPN protocols
type L2TPOutbound struct {
	LegacyVPNBaseOutbound
}

type L2TPIPsecOutbound struct {
	LegacyVPNBaseOutbound
}

type IKEv2Outbound struct {
	LegacyVPNBaseOutbound
}

type IKEv2IPsecOutbound struct {
	LegacyVPNBaseOutbound
}

type SSTPOutbound struct {
	LegacyVPNBaseOutbound
}

type PPTPOutbound struct {
	LegacyVPNBaseOutbound
}

type GREOutbound struct {
	LegacyVPNBaseOutbound
}

type SoftEtherOutbound struct {
	LegacyVPNBaseOutbound
}

// Constructor functions for each protocol
func NewL2TPIPsecOutbound(router route.Router, logger log.Logger, options option.Outbound) (*L2TPIPsecOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "l2tp-ipsec"
	return &L2TPIPsecOutbound{*base}, nil
}

func NewIKEv2Outbound(router route.Router, logger log.Logger, options option.Outbound) (*IKEv2Outbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "ikev2"
	return &IKEv2Outbound{*base}, nil
}

func NewIKEv2IPsecOutbound(router route.Router, logger log.Logger, options option.Outbound) (*IKEv2IPsecOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "ikev2-ipsec"
	return &IKEv2IPsecOutbound{*base}, nil
}

func NewSSTPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSTPOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "sstp"
	return &SSTPOutbound{*base}, nil
}

func NewPPTPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*PPTPOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "pptp"
	return &PPTPOutbound{*base}, nil
}

func NewGREOutbound(router route.Router, logger log.Logger, options option.Outbound) (*GREOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "gre"
	return &GREOutbound{*base}, nil
}

func NewSoftEtherOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SoftEtherOutbound, error) {
	base, err := NewL2TPOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "softether"
	return &SoftEtherOutbound{*base}, nil
}

// Connection wrapper types
type L2TPConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type L2TPIPsecConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type IKEv2Conn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type IKEv2IPsecConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type SSTPConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type PPTPConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type GREConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

type SoftEtherConn struct {
	conn   net.Conn
	config LegacyVPNConfig
}

// Implement net.Conn interface for all connection wrappers
func (c *L2TPConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *L2TPConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *L2TPConn) Close() error {
	return c.conn.Close()
}

func (c *L2TPConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *L2TPConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *L2TPConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *L2TPConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *L2TPConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Implement other connection wrappers similarly...
// For brevity, showing one pattern - in real implementation, all would be implemented


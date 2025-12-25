package psiphon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

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
	register.Outbound("psiphon", NewPsiphonOutbound)
	register.Outbound("psiphon-ssh", NewPsiphonSSHOutbound)
	register.Outbound("psiphon-quic-go", NewPsiphonQuicOutbound)
	register.Outbound("psiphon-meek", NewPsiphonMeekOutbound)
	register.Outbound("psiphon-obfs3", NewPsiphonObfs3Outbound)
}

type PsiphonConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // "ssh", "quic-go", "meek", "obfs3", "fronting-meek"
	ServerEntry    string `json:"server_entry"` // Server entry ID
	ClientID       string `json:"client_id"`
	AuthToken      string `json:"auth_token"`
	UseToken       bool   `json:"use_token"`
	SSHConfig      SSHConfig `json:"ssh_config"`
	TLSConfig      TLSConfig `json:"tls_config"`
	MeekConfig     MeekConfig `json:"meek_config"`
	QUICConfig     QUICConfig `json:"quic_config"`
	FrontingDomain string `json:"fronting_domain"` // For fronting
	FrontingURL    string `json:"fronting_url"`    // For fronting
	// Psiphon-specific
	Capabilities   []string `json:"capabilities"` // ["SSH", "OSSH", "QuicGo", "Meek", "Obfs3"]
	SSHUsername    string   `json:"ssh_username"`
	SSHPassword    string   `json:"ssh_password"`
	MeekEndpoint  string   `json:"meek_endpoint"`
	// Connection settings
	KeepAlive      int      `json:"keep_alive"` // Keep alive interval
	RetryCount     int      `json:"retry_count"`
	RetryDelay     int      `json:"retry_delay"` // Retry delay in seconds
	// Performance settings
	BufferSize     int      `json:"buffer_size"`
	Compress       bool     `json:"compress"`
}

type SSHConfig struct {
	HostKey      string `json:"host_key"`
	HostKeyAlgo  string `json:"host_key_algo"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	PrivateKey   string `json:"private_key"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type MeekConfig struct {
	Amazons3URL   string `json:"amazons3_url"`
	AzureURL      string `json:"azure_url"`
	GoogleURL     string `json:"google_url"`
	MaxRetries    int    `json:"max_retries"`
	Strategy      string `json:"strategy"` // "random", "sequential"
}

type QUICConfig struct {
	MaxIdleTimeout  time.Duration `json:"max_idle_timeout"`
	MaxReceiveStream int          `json:"max_receive_stream"`
	MaxReceivePacket int          `json:"max_receive_packet"`
	DisablePathMTU  bool          `json:"disable_path_mtu"`
}

type PsiphonBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     PsiphonConfig
	connection net.Conn
}

func NewPsiphonOutbound(router route.Router, logger log.Logger, options option.Outbound) (*PsiphonOutbound, error) {
	var config PsiphonConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	// Set default Psiphon configuration
	if config.Server == "" {
		config.Server = "entry.psiphon3.com"
	}
	if config.Port == 0 {
		config.Port = 443
	}
	if config.Protocol == "" {
		config.Protocol = "meek"
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5
	}

	return &PsiphonOutbound{
		PsiphonBaseOutbound: PsiphonBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (p *PsiphonBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return p.connectAndRoute(ctx, packet)
}

func (p *PsiphonBaseOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if p.connection == nil {
		conn, err := p.createConnection()
		if err != nil {
			return err
		}
		p.connection = conn
	}

	// Route packet through Psiphon tunnel
	_, err := p.connection.Write(packet.Data())
	return err
}

func (p *PsiphonBaseOutbound) createConnection() (net.Conn, error) {
	var conn net.Conn
	var err error

	switch p.config.Protocol {
	case "ssh":
		conn, err = p.createSSHConnection()
	case "meek":
		conn, err = p.createMeekConnection()
	case "fronting-meek":
		conn, err = p.createFrontingMeekConnection()
	case "quic-go":
		conn, err = p.createQUICConnection()
	case "obfs3":
		conn, err = p.createObfs3Connection()
	default:
		conn, err = p.createMeekConnection()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Psiphon %s connection: %v", p.config.Protocol, err)
	}

	return conn, nil
}

func (p *PsiphonBaseOutbound) createSSHConnection() (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)
	
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// Perform SSH handshake with Psiphon server
	if err := p.performSSHHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PsiphonSSHConn{conn: conn, config: p.config}, nil
}

func (p *PsiphonBaseOutbound) createMeekConnection() (net.Conn, error) {
	// Meek connection through Psiphon
	serverURL := p.config.MeekEndpoint
	if serverURL == "" {
		serverURL = "https://meek.azureedge.net/"
	}

	// Create HTTPS connection to meek server
	tlsConfig := &tls.Config{
		ServerName: "meek.azureedge.net",
	}

	conn, err := tls.Dial("tcp", "meek.azureedge.net:443", tlsConfig)
	if err != nil {
		return nil, err
	}

	// Perform Psiphon meek handshake
	if err := p.performMeekHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PsiphonMeekConn{conn: conn, config: p.config}, nil
}

func (p *PsiphonBaseOutbound) createFrontingMeekConnection() (net.Conn, error) {
	// Meek with fronting domain
	frontingDomain := p.config.FrontingDomain
	if frontingDomain == "" {
		frontingDomain = "cloudflare.com"
	}

	tlsConfig := &tls.Config{
		ServerName: frontingDomain,
	}

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", frontingDomain), tlsConfig)
	if err != nil {
		return nil, err
	}

	if err := p.performFrontingMeekHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PsiphonFrontingMeekConn{conn: conn, config: p.config, frontingDomain: frontingDomain}, nil
}

func (p *PsiphonBaseOutbound) createQUICConnection() (net.Conn, error) {
	// QUIC connection through Psiphon
	address := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)
	
	// Create QUIC connection (simplified)
	// In real implementation, would use quic-go library
	
	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	if err := p.performQUICHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PsiphonQUICConn{conn: conn, config: p.config}, nil
}

func (p *PsiphonBaseOutbound) createObfs3Connection() (net.Conn, error) {
	// Obfs3 obfuscated connection
	address := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)
	
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if err := p.performObfs3Handshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &PsiphonObfs3Conn{conn: conn, config: p.config}, nil
}

func (p *PsiphonBaseOutbound) performSSHHandshake(conn net.Conn) error {
	p.logger.Info("Performing Psiphon SSH handshake")
	
	// SSH protocol exchange
	_, err := conn.Write([]byte("SSH-2.0-Psiphon\r\n"))
	if err != nil {
		return err
	}

	// Read server version
	// In real implementation, would perform full SSH authentication
	
	return nil
}

func (p *PsiphonBaseOutbound) performMeekHandshake(conn net.Conn) error {
	p.logger.Info("Performing Psiphon meek handshake")
	
	// Send meek handshake request
	// This includes Psiphon client ID and authentication
	
	return nil
}

func (p *PsiphonBaseOutbound) performFrontingMeekHandshake(conn net.Conn) error {
	p.logger.Info("Performing Psiphon fronting meek handshake")
	
	// Send fronted request that appears to be for normal website
	// but actually tunnels Psiphon traffic
	
	return nil
}

func (p *PsiphonBaseOutbound) performQUICHandshake(conn net.Conn) error {
	p.logger.Info("Performing Psiphon QUIC handshake")
	
	// Perform QUIC handshake for Psiphon
	
	return nil
}

func (p *PsiphonBaseOutbound) performObfs3Handshake(conn net.Conn) error {
	p.logger.Info("Performing Psiphon Obfs3 handshake")
	
	// Perform Obfs3 obfuscation handshake
	
	return nil
}

func (p *PsiphonBaseOutbound) Close() error {
	if p.connection != nil {
		return p.connection.Close()
	}
	return nil
}

func (p *PsiphonBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different Psiphon protocols
type PsiphonOutbound struct {
	PsiphonBaseOutbound
}

type PsiphonSSHOutbound struct {
	PsiphonBaseOutbound
}

type PsiphonQuicOutbound struct {
	PsiphonBaseOutbound
}

type PsiphonMeekOutbound struct {
	PsiphonBaseOutbound
}

type PsiphonObfs3Outbound struct {
	PsiphonBaseOutbound
}

// Constructor functions for each Psiphon protocol
func NewPsiphonSSHOutbound(router route.Router, logger log.Logger, options option.Outbound) (*PsiphonSSHOutbound, error) {
	base, err := NewPsiphonOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "ssh"
	return &PsiphonSSHOutbound{*base}, nil
}

func NewPsiphonQuicOutbound(router route.Router, logger log.Logger, options option.Outbound) (*PsiphonQuicOutbound, error) {
	base, err := NewPsiphonOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "quic-go"
	return &PsiphonQuicOutbound{*base}, nil
}

func NewPsiphonMeekOutbound(router route.Router, logger log.Logger, options option.Outbound) (*PsiphonMeekOutbound, error) {
	base, err := NewPsiphonOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "meek"
	return &PsiphonMeekOutbound{*base}, nil
}

func NewPsiphonObfs3Outbound(router route.Router, logger log.Logger, options option.Outbound) (*PsiphonObfs3Outbound, error) {
	base, err := NewPsiphonOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "obfs3"
	return &PsiphonObfs3Outbound{*base}, nil
}

// Connection wrapper types
type PsiphonSSHConn struct {
	conn   net.Conn
	config PsiphonConfig
}

type PsiphonMeekConn struct {
	conn   net.Conn
	config PsiphonConfig
}

type PsiphonFrontingMeekConn struct {
	conn          net.Conn
	config        PsiphonConfig
	frontingDomain string
}

type PsiphonQUICConn struct {
	conn   net.Conn
	config PsiphonConfig
}

type PsiphonObfs3Conn struct {
	conn   net.Conn
	config PsiphonConfig
}

// Implement net.Conn interface for all connection wrappers
func (c *PsiphonSSHConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *PsiphonSSHConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *PsiphonSSHConn) Close() error {
	return c.conn.Close()
}

func (c *PsiphonSSHConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *PsiphonSSHConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *PsiphonSSHConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *PsiphonSSHConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *PsiphonSSHConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Implement other connection wrappers similarly...
func (c *PsiphonMeekConn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *PsiphonMeekConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *PsiphonMeekConn) Close() error {
	return c.conn.Close()
}

func (c *PsiphonMeekConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *PsiphonMeekConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *PsiphonMeekConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *PsiphonMeekConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *PsiphonMeekConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}


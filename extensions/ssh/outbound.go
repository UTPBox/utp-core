package ssh

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
	register.Outbound("ssh-direct", NewSSHDirectOutbound)
	register.Outbound("ssh-proxy", NewSSHProxyOutbound)
	register.Outbound("ssh-payload", NewSSHPayloadOutbound)
	register.Outbound("ssh-proxy-payload", NewSSHProxyPayloadOutbound)
	register.Outbound("ssh-tls", NewSSHTLSOutbound)
	register.Outbound("ssh-tls-proxy", NewSSHTLSProxyOutbound)
	register.Outbound("ssh-tls-payload", NewSSHTLSPayloadOutbound)
	register.Outbound("ssh-tls-proxy-payload", NewSSHTLSProxyPayloadOutbound)
	register.Outbound("ssh-dnstt", NewSSHDNSTTOutbound)
}

type SSHConfig struct {
	Server       string `json:"server"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	PrivateKey   string `json:"private_key"`
	HostKey      string `json:"host_key"`
	Method       string `json:"method"` // "direct", "proxy", "payload", etc.
	TLS          bool   `json:"tls"`
	TLSConfig    TLSConfig `json:"tls_config"`
	ProxyServer  string `json:"proxy_server"`
	ProxyPort    int    `json:"proxy_port"`
	ProxyMethod  string `json:"proxy_method"` // "http", "socks5"
	Payload      string `json:"payload"`
	DNSOverHTTPS string `json:"dns_over_https"` // DNS-over-HTTPS server for DNSTT
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type SSHBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     SSHConfig
	connection net.Conn
	tlsConn    net.Conn
}

func NewSSHDirectOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHDirectOutbound, error) {
	var config SSHConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	return &SSHDirectOutbound{
		SSHBaseOutbound: SSHBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (s *SSHBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return s.connectAndRoute(ctx, packet)
}

func (s *SSHBaseOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if s.connection == nil {
		conn, err := s.createConnection()
		if err != nil {
			return err
		}
		s.connection = conn
	}

	_, err := s.connection.Write(packet.Data())
	return err
}

func (s *SSHBaseOutbound) createConnection() (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", s.config.Server, s.config.Port)
	
	var conn net.Conn
	var err error

	// Handle different SSH variants
	switch s.config.Method {
	case "direct":
		conn, err = net.Dial("tcp", address)
	case "proxy":
		conn, err = s.connectViaProxy(address)
	case "payload":
		conn, err = s.connectViaPayload(address)
	case "proxy-payload":
		conn, err = s.connectViaProxyPayload(address)
	case "tls":
		conn, err = s.connectViaTLS(address)
	case "tls-proxy":
		conn, err = s.connectViaTLSProxy(address)
	case "tls-payload":
		conn, err = s.connectViaTLSPayload(address)
	case "tls-proxy-payload":
		conn, err = s.connectViaTLSProxyPayload(address)
	case "dnstt":
		conn, err = s.connectViaDNSTT(address)
	default:
		conn, err = net.Dial("tcp", address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	// Perform SSH handshake
	if err := s.performSSHHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (s *SSHBaseOutbound) connectViaProxy(address string) (net.Conn, error) {
	proxyAddr := fmt.Sprintf("%s:%d", s.config.ProxyServer, s.config.ProxyPort)
	
	proxyConn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// Send HTTP CONNECT or SOCKS5 handshake
	if s.config.ProxyMethod == "http" {
		_, err = proxyConn.Write([]byte(fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", address, address)))
	} else {
		// SOCKS5 handshake (simplified)
		proxyConn.Write([]byte{0x05, 0x01, 0x00})
		proxyConn.Write([]byte{0x05, 0x01, 0x00, 0x03})
	}

	if err != nil {
		proxyConn.Close()
		return nil, err
	}

	return proxyConn, nil
}

func (s *SSHBaseOutbound) connectViaPayload(address string) (net.Conn, error) {
	// Create connection and inject payload
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if s.config.Payload != "" {
		_, err = conn.Write([]byte(s.config.Payload))
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (s *SSHBaseOutbound) connectViaProxyPayload(address string) (net.Conn, error) {
	conn, err := s.connectViaProxy(address)
	if err != nil {
		return nil, err
	}

	if s.config.Payload != "" {
		_, err = conn.Write([]byte(s.config.Payload))
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (s *SSHBaseOutbound) connectViaTLS(address string) (net.Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName:         s.config.TLSConfig.ServerName,
		InsecureSkipVerify: s.config.TLSConfig.Insecure,
	}

	if s.config.TLSConfig.CA != "" {
		// Load CA certificate
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	s.tlsConn = tlsConn
	return tlsConn, nil
}

func (s *SSHBaseOutbound) connectViaTLSProxy(address string) (net.Conn, error) {
	proxyConn, err := s.connectViaProxy(address)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ServerName:         s.config.TLSConfig.ServerName,
		InsecureSkipVerify: s.config.TLSConfig.Insecure,
	}

	tlsConn := tls.Client(proxyConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		proxyConn.Close()
		return nil, err
	}

	s.tlsConn = tlsConn
	return tlsConn, nil
}

func (s *SSHBaseOutbound) connectViaTLSPayload(address string) (net.Conn, error) {
	conn, err := s.connectViaTLS(address)
	if err != nil {
		return nil, err
	}

	if s.config.Payload != "" {
		_, err = conn.Write([]byte(s.config.Payload))
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (s *SSHBaseOutbound) connectViaTLSProxyPayload(address string) (net.Conn, error) {
	conn, err := s.connectViaTLSProxy(address)
	if err != nil {
		return nil, err
	}

	if s.config.Payload != "" {
		_, err = conn.Write([]byte(s.config.Payload))
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

func (s *SSHBaseOutbound) connectViaDNSTT(address string) (net.Conn, error) {
	// DNS-over-Tunnel implementation - tunnel SSH over DNS queries
	if s.config.DNSOverHTTPS == "" {
		return nil, fmt.Errorf("DNS-over-HTTPS server required for DNSTT")
	}

	// This is a simplified DNSTT implementation
	// In reality, this would encode SSH traffic into DNS queries
	s.logger.Info("Using DNS-over-Tunnel for SSH connection")
	
	// Create a mock connection for now
	// Real implementation would use DNS-over-HTTPS or direct DNS
	return &mockConn{}, nil
}

func (s *SSHBaseOutbound) performSSHHandshake(conn net.Conn) error {
	s.logger.Info("Performing SSH handshake")
	
	// SSH protocol version exchange
	_, err := conn.Write([]byte("SSH-2.0-UTP-Core_1.0\r\n"))
	if err != nil {
		return err
	}

	// Read server version
	// In real implementation, would perform full SSH handshake
	return nil
}

func (s *SSHBaseOutbound) Close() error {
	if s.connection != nil {
		return s.connection.Close()
	}
	return nil
}

func (s *SSHBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different SSH variants
type SSHDirectOutbound struct {
	SSHBaseOutbound
}

type SSHProxyOutbound struct {
	SSHBaseOutbound
}

type SSHPayloadOutbound struct {
	SSHBaseOutbound
}

type SSHProxyPayloadOutbound struct {
	SSHBaseOutbound
}

type SSHTLSOutbound struct {
	SSHBaseOutbound
}

type SSHTLSProxyOutbound struct {
	SSHBaseOutbound
}

type SSHTLSPayloadOutbound struct {
	SSHBaseOutbound
}

type SSHTLSProxyPayloadOutbound struct {
	SSHBaseOutbound
}

type SSHDNSTTOutbound struct {
	SSHBaseOutbound
}

// Constructor functions for each variant
func NewSSHProxyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHProxyOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHProxyOutbound{*base}, nil
}

func NewSSHPayloadOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHPayloadOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHPayloadOutbound{*base}, nil
}

func NewSSHProxyPayloadOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHProxyPayloadOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHProxyPayloadOutbound{*base}, nil
}

func NewSSHTLSOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHTLSOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHTLSOutbound{*base}, nil
}

func NewSSHTLSProxyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHTLSProxyOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHTLSProxyOutbound{*base}, nil
}

func NewSSHTLSPayloadOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHTLSPayloadOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHTLSPayloadOutbound{*base}, nil
}

func NewSSHTLSProxyPayloadOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHTLSProxyPayloadOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHTLSProxyPayloadOutbound{*base}, nil
}

func NewSSHDNSTTOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SSHDNSTTOutbound, error) {
	base, err := NewSSHDirectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SSHDNSTTOutbound{*base}, nil
}

// Mock connection for DNSTT
type mockConn struct{}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, fmt.Errorf("mock connection - not implemented")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConn) Close() error {
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}


package obfs

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
	register.Outbound("obfs4", NewObfs4Outbound)
	register.Outbound("meek", NewMeekOutbound)
	register.Outbound("naiveproxy", NewNaiveProxyOutbound)
	register.Outbound("udp2raw", NewUDP2RAWOutbound)
	register.Outbound("cloak", NewCloakOutbound)
	register.Outbound("fteproxy", NewFTEProxyOutbound)
	register.Outbound("scramblesuit", NewScrambleSuitOutbound)
	register.Outbound("snowflake", NewSnowflakeOutbound)
}

type ObfsConfig struct {
	Server           string `json:"server"`
	Port             int    `json:"port"`
	Protocol         string `json:"protocol"` // "obfs4", "meek", "naiveproxy", "udp2raw", "cloak", "fteproxy", "scramblesuit", "snowflake"
	// Obfs4 specific
	PublicKey        string `json:"public_key"`
	PrivateKey       string `json:"private_key"`
	NodeID           string `json:"node_id"`
	Password         string `json:"password"`
	// Meek specific
	FrontingDomain   string `json:"fronting_domain"`
	FrontingURL      string `json:"fronting_url"`
	// NaiveProxy specific
	NaiveMethod      string `json:"naive_method"`
	NaivePassword    string `json:"naive_password"`
	// Cloak specific
	CloakKey         string `json:"cloak_key"`
	CloakBlueprint   string `json:"cloak_blueprint"`
	// FTE specific
	FTEKey           string `json:"fte_key"`
	FTEMode          string `json:"fte_mode"`
	// Snowflake specific
	SnowflakeBroker  string `json:"snowflake_broker"`
	SnowflakeToken   string `json:"snowflake_token"`
	// Common
	TLSConfig        TLSConfig `json:"tls_config"`
	Timeout          int       `json:"timeout"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type ObfsBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     ObfsConfig
	connection net.Conn
}

func NewObfs4Outbound(router route.Router, logger log.Logger, options option.Outbound) (*Obfs4Outbound, error) {
	var config ObfsConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	return &Obfs4Outbound{
		ObfsBaseOutbound: ObfsBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (o *ObfsBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return o.connectAndRoute(ctx, packet)
}

func (o *ObfsBaseOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if o.connection == nil {
		conn, err := o.createConnection()
		if err != nil {
			return err
		}
		o.connection = conn
	}

	// Obfuscate and send packet
	obfuscatedData, err := o.obfuscateData(packet.Data())
	if err != nil {
		return fmt.Errorf("obfuscation failed: %v", err)
	}

	_, err = o.connection.Write(obfuscatedData)
	return err
}

func (o *ObfsBaseOutbound) createConnection() (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", o.config.Server, o.config.Port)
	
	var conn net.Conn
	var err error

	switch o.config.Protocol {
	case "obfs4":
		conn, err = o.createObfs4Connection(address)
	case "meek":
		conn, err = o.createMeekConnection(address)
	case "naiveproxy":
		conn, err = o.createNaiveProxyConnection(address)
	case "udp2raw":
		conn, err = o.createUDP2RAWConnection(address)
	case "cloak":
		conn, err = o.createCloakConnection(address)
	case "fteproxy":
		conn, err = o.createFTEProxyConnection(address)
	case "scramblesuit":
		conn, err = o.createScrambleSuitConnection(address)
	case "snowflake":
		conn, err = o.createSnowflakeConnection(address)
	default:
		return nil, fmt.Errorf("unsupported obfuscation protocol: %s", o.config.Protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s connection: %v", o.config.Protocol, err)
	}

	return conn, nil
}

func (o *ObfsBaseOutbound) createObfs4Connection(address string) (net.Conn, error) {
	// Create Obfs4 connection with bridge authentication
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	// Perform Obfs4 handshake
	if err := o.performObfs4Handshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &Obfs4Conn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createMeekConnection(address string) (net.Conn, error) {
	// Meek uses HTTP(S) with fronting domain
	frontingURL := o.config.FrontingURL
	if frontingURL == "" {
		frontingURL = fmt.Sprintf("https://%s:%d", o.config.Server, o.config.Port)
	}

	// Create HTTPS connection to fronting domain
	tlsConfig := &tls.Config{
		ServerName:         o.config.FrontingDomain,
		InsecureSkipVerify: o.config.TLSConfig.Insecure,
	}

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", o.config.FrontingDomain), tlsConfig)
	if err != nil {
		return nil, err
	}

	return &MeekConn{conn: conn, config: o.config, frontingURL: frontingURL}, nil
}

func (o *ObfsBaseOutbound) createNaiveProxyConnection(address string) (net.Conn, error) {
	// NaiveProxy uses HTTPS with method obfuscation
	tlsConfig := &tls.Config{
		ServerName: o.config.Server,
	}

	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, err
	}

	// Perform NaiveProxy authentication
	if err := o.performNaiveProxyHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &NaiveProxyConn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createUDP2RAWConnection(address string) (net.Conn, error) {
	// UDP2RAW tunnels UDP over TCP
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	return &UDP2RAWConn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createCloakConnection(address string) (net.Conn, error) {
	// Cloak uses protocol obfuscation
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if err := o.performCloakHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &CloakConn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createFTEProxyConnection(address string) (net.Conn, error) {
	// FTEProxy uses Format-Transforming Encryption
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if err := o.performFTEProxyHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &FTEProxyConn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createScrambleSuitConnection(address string) (net.Conn, error) {
	// ScrambleSuit uses probabilistic encryption
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if err := o.performScrambleSuitHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &ScrambleSuitConn{conn: conn, config: o.config}, nil
}

func (o *ObfsBaseOutbound) createSnowflakeConnection(address string) (net.Conn, error) {
	// Snowflake uses broker-based connection
	brokerURL := o.config.SnowflakeBroker
	
	// This would connect to a Snowflake broker and get a proxy
	// For now, create a mock connection
	o.logger.Info("Using Snowflake broker:", brokerURL)
	
	return &SnowflakeConn{config: o.config}, nil
}

func (o *ObfsBaseOutbound) obfuscateData(data []byte) ([]byte, error) {
	switch o.config.Protocol {
	case "obfs4":
		return o.obfuscateObfs4(data)
	case "meek":
		return o.obfuscateMeek(data)
	case "naiveproxy":
		return o.obfuscateNaiveProxy(data)
	case "udp2raw":
		return o.obfuscateUDP2RAW(data)
	case "cloak":
		return o.obfuscateCloak(data)
	case "fteproxy":
		return o.obfuscateFTEProxy(data)
	case "scramblesuit":
		return o.obfuscateScrambleSuit(data)
	case "snowflake":
		return o.obfuscateSnowflake(data)
	default:
		return data, nil
	}
}

func (o *ObfsBaseOutbound) deobfuscateData(data []byte) ([]byte, error) {
	switch o.config.Protocol {
	case "obfs4":
		return o.deobfuscateObfs4(data)
	case "meek":
		return o.deobfuscateMeek(data)
	case "naiveproxy":
		return o.deobfuscateNaiveProxy(data)
	case "udp2raw":
		return o.deobfuscateUDP2RAW(data)
	case "cloak":
		return o.deobfuscateCloak(data)
	case "fteproxy":
		return o.deobfuscateFTEProxy(data)
	case "scramblesuit":
		return o.deobfuscateScrambleSuit(data)
	case "snowflake":
		return o.deobfuscateSnowflake(data)
	default:
		return data, nil
	}
}

// Simplified obfuscation methods (in real implementation, these would use actual algorithms)
func (o *ObfsBaseOutbound) obfuscateObfs4(data []byte) ([]byte, error) {
	// Obfs4 uses authenticated encryption
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateObfs4(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateMeek(data []byte) ([]byte, error) {
	// Meek uses HTTP(S) for transport
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateMeek(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateNaiveProxy(data []byte) ([]byte, error) {
	// NaiveProxy uses HTTPS with method obfuscation
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateNaiveProxy(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateUDP2RAW(data []byte) ([]byte, error) {
	// UDP2RAW converts UDP to TCP
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateUDP2RAW(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateCloak(data []byte) ([]byte, error) {
	// Cloak uses protocol obfuscation
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateCloak(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateFTEProxy(data []byte) ([]byte, error) {
	// FTE uses Format-Transforming Encryption
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateFTEProxy(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateScrambleSuit(data []byte) ([]byte, error) {
	// ScrambleSuit uses probabilistic encryption
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateScrambleSuit(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) obfuscateSnowflake(data []byte) ([]byte, error) {
	// Snowflake uses broker-based transport
	return data, nil
}

func (o *ObfsBaseOutbound) deobfuscateSnowflake(data []byte) ([]byte, error) {
	return data, nil
}

func (o *ObfsBaseOutbound) performObfs4Handshake(conn net.Conn) error {
	// Perform Obfs4 handshake with authentication
	return nil
}

func (o *ObfsBaseOutbound) performNaiveProxyHandshake(conn net.Conn) error {
	// Perform NaiveProxy handshake
	return nil
}

func (o *ObfsBaseOutbound) performCloakHandshake(conn net.Conn) error {
	// Perform Cloak handshake
	return nil
}

func (o *ObfsBaseOutbound) performFTEProxyHandshake(conn net.Conn) error {
	// Perform FTEProxy handshake
	return nil
}

func (o *ObfsBaseOutbound) performScrambleSuitHandshake(conn net.Conn) error {
	// Perform ScrambleSuit handshake
	return nil
}

func (o *ObfsBaseOutbound) Close() error {
	if o.connection != nil {
		return o.connection.Close()
	}
	return nil
}

func (o *ObfsBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different obfuscation protocols
type Obfs4Outbound struct {
	ObfsBaseOutbound
}

type MeekOutbound struct {
	ObfsBaseOutbound
}

type NaiveProxyOutbound struct {
	ObfsBaseOutbound
}

type UDP2RAWOutbound struct {
	ObfsBaseOutbound
}

type CloakOutbound struct {
	ObfsBaseOutbound
}

type FTEProxyOutbound struct {
	ObfsBaseOutbound
}

type ScrambleSuitOutbound struct {
	ObfsBaseOutbound
}

type SnowflakeOutbound struct {
	ObfsBaseOutbound
}

// Constructor functions for each protocol
func NewMeekOutbound(router route.Router, logger log.Logger, options option.Outbound) (*MeekOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &MeekOutbound{*base}, nil
}

func NewNaiveProxyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*NaiveProxyOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &NaiveProxyOutbound{*base}, nil
}

func NewUDP2RAWOutbound(router route.Router, logger log.Logger, options option.Outbound) (*UDP2RAWOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &UDP2RAWOutbound{*base}, nil
}

func NewCloakOutbound(router route.Router, logger log.Logger, options option.Outbound) (*CloakOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &CloakOutbound{*base}, nil
}

func NewFTEProxyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*FTEProxyOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &FTEProxyOutbound{*base}, nil
}

func NewScrambleSuitOutbound(router route.Router, logger log.Logger, options option.Outbound) (*ScrambleSuitOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &ScrambleSuitOutbound{*base}, nil
}

func NewSnowflakeOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SnowflakeOutbound, error) {
	base, err := NewObfs4Outbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SnowflakeOutbound{*base}, nil
}

// Connection wrapper types
type Obfs4Conn struct {
	conn    net.Conn
	config  ObfsConfig
}

type MeekConn struct {
	conn        net.Conn
	config      ObfsConfig
	frontingURL string
}

type NaiveProxyConn struct {
	conn   net.Conn
	config ObfsConfig
}

type UDP2RAWConn struct {
	conn   net.Conn
	config ObfsConfig
}

type CloakConn struct {
	conn   net.Conn
	config ObfsConfig
}

type FTEProxyConn struct {
	conn   net.Conn
	config ObfsConfig
}

type ScrambleSuitConn struct {
	conn   net.Conn
	config ObfsConfig
}

type SnowflakeConn struct {
	config ObfsConfig
}

// Implement net.Conn interface for all connection wrappers
func (c *Obfs4Conn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *Obfs4Conn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *Obfs4Conn) Close() error {
	return c.conn.Close()
}

func (c *Obfs4Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Obfs4Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Obfs4Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Obfs4Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Obfs4Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Implement the remaining conn wrappers similarly...
// For brevity, showing one pattern - in real implementation, all would be implemented


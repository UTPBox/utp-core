package openvpn

import (
	"context"
	"fmt"
	"net"

	"github.com/sagernet/sing-box/common/register"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing-tunnel"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/x"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/quickclose"
)

func init() {
	register.Outbound("openvpn", NewOpenVPNOutbound)
}

type OpenVPNConfig struct {
	Server      string `json:"server"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"` // "tcp" or "udp"
	CA          string `json:"ca"`
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	TLSAuth     string `json:"tls_auth"`
	TLSAuthKey  string `json:"tls_auth_key"`
	Encryption  string `json:"encryption"` // cipher type
	Compression bool   `json:"compression"`
	Verbosity   int    `json:"verbosity"`
}

type OpenVPNOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     OpenVPNConfig
	connection net.Conn
}

func NewOpenVPNOutbound(router route.Router, logger log.Logger, options option.Outbound) (*OpenVPNOutbound, error) {
	var config OpenVPNConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("missing config")
	}

	return &OpenVPNOutbound{
		ctx:    context.Background(),
		logger: logger,
		router: router,
		config: config,
	}, nil
}

func (o *OpenVPNOutbound) Route(ctx context.Context, packet routing.Packet) error {
	// Route packets through OpenVPN connection
	return o.connectAndRoute(ctx, packet)
}

func (o *OpenVPNOutbound) connectAndRoute(ctx context.Context, packet routing.Packet) error {
	if o.connection == nil {
		conn, err := o.createConnection()
		if err != nil {
			return err
		}
		o.connection = conn
	}

	// Forward packet through OpenVPN connection
	_, err := o.connection.Write(packet.Data())
	return err
}

func (o *OpenVPNOutbound) createConnection() (net.Conn, error) {
	var conn net.Conn
	var err error

	address := fmt.Sprintf("%s:%d", o.config.Server, o.config.Port)

	if o.config.Protocol == "tcp" {
		conn, err = net.Dial("tcp", address)
	} else {
		conn, err = net.Dial("udp", address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to OpenVPN server: %v", err)
	}

	// Perform OpenVPN handshake (simplified)
	if err := o.performHandshake(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (o *OpenVPNOutbound) performHandshake(conn net.Conn) error {
	// Simplified OpenVPN handshake - in reality this would be more complex
	o.logger.Info("Performing OpenVPN handshake")
	
	// Send TLS handshake, exchange certificates, etc.
	// This is a stub implementation
	
	return nil
}

func (o *OpenVPNOutbound) Close() error {
	if o.connection != nil {
		return o.connection.Close()
	}
	return nil
}

func (o *OpenVPNOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

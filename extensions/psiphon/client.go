package psiphon

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/metadata"
	"golang.org/x/crypto/ssh"
)

var _ adapter.Outbound = (*Outbound)(nil)

type Outbound struct {
	tag  string
	opts PsiphonOptions
}

// NewOutbound creates a new Psiphon outbound
func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, rawMessage json.RawMessage) (adapter.Outbound, error) {
	var opts PsiphonOptions
	if err := json.Unmarshal(rawMessage, &opts); err != nil {
		return nil, err
	}
	return &Outbound{
		tag:  tag,
		opts: opts,
	}, nil
}

func (o *Outbound) Type() string {
	return "psiphon"
}

func (o *Outbound) Tag() string {
	return o.tag
}

func (o *Outbound) Dependencies() []string {
	return nil
}

func (o *Outbound) Start() error {
	return nil
}

func (o *Outbound) Close() error {
	return nil
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination metadata.Socksaddr) (net.Conn, error) {
	// 1. Dial base TCP connection to the Psiphon server
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", o.opts.Server, o.opts.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to dial server: %w", err)
	}

	// 2. Wrap with TLS if configured
	if o.opts.UseTLS {
		tlsConfig := &tls.Config{
			ServerName: o.opts.HeaderHost,
			InsecureSkipVerify: true,
		}
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = o.opts.Server
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		conn = tlsConn
	}

	// 3. Perform HTTP Handshake
	if err := doHTTPHandshake(conn, o.opts); err != nil {
		conn.Close()
		return nil, fmt.Errorf("HTTP handshake failed: %w", err)
	}

	// 4. Establish SSH Session
	sshConfig := &ssh.ClientConfig{
		User: o.opts.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(o.opts.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         C.TCPTimeout,
	}

	// Establish SSH connection
	sshConn, channels, reqs, err := ssh.NewClientConn(conn, o.opts.Server, sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	// Create SSH client
	sshClient := ssh.NewClient(sshConn, channels, reqs)

	// 5. Dial target
	targetAddr := destination.String()
	proxyConn, err := sshClient.Dial(network, targetAddr)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to dial target via SSH: %w", err)
	}

	return proxyConn, nil
}

func (o *Outbound) DialPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	return nil, fmt.Errorf("UDP not supported in this basic Psiphon implementation")
}

// Implement ListenPacket to satisfy interface
func (o *Outbound) ListenPacket(ctx context.Context, destination metadata.Socksaddr) (net.PacketConn, error) {
	// Usually used for inbound UDP connection handling? 
	// Or maybe for specific reverse tunneling?
	return nil, fmt.Errorf("ListenPacket not supported in Psiphon output")
}

// Implement Network() method
func (o *Outbound) Network() []string {
	return []string{"tcp"}
}

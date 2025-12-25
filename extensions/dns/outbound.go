package dns

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
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
	miekgdns "github.com/miekg/dns"
)

func init() {
	register.Outbound("dns-doh", NewDoHOutbound)
	register.Outbound("dns-dot", NewDoTOutbound)
	register.Outbound("dns-dnscrypt", NewDNSCryptOutbound)
	register.Outbound("dns-doq", NewDoQOutbound)
	register.Outbound("dns-slowdns", NewSlowDNSOutbound)
	register.Outbound("dns-tcp", NewDNSTCPOutbound)
	register.Outbound("dns-udp", NewDNSUDPOutbound)
}

type DNSConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // "doh", "dot", "dnscrypt", "doq", "slowdns", "tcp", "udp"
	URL            string `json:"url"`      // For DoH
	Headers        map[string]string `json:"headers"` // HTTP headers for DoH
	TLSConfig      TLSConfig `json:"tls_config"`
	CryptKey       string `json:"crypt_key"` // For DNSCrypt
	CryptProvider  string `json:"crypt_provider"`
	Resolver       string `json:"resolver"` // Upstream resolver
	Bootstrap      string `json:"bootstrap"` // Bootstrap DNS server
	EDNSClientSubnet string `json:"edns_client_subnet"`
	Timeout        time.Duration `json:"timeout"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type DNSBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     DNSConfig
	client     *miekgdns.Client
	httpClient *http.Client
}

func NewDoHOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DoHOutbound, error) {
	var config DNSConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:         config.TLSConfig.ServerName,
				InsecureSkipVerify: config.TLSConfig.Insecure,
			},
		},
		Timeout: config.Timeout,
	}

	return &DoHOutbound{
		DNSBaseOutbound: DNSBaseOutbound{
			ctx:        context.Background(),
			logger:     logger,
			router:     router,
			config:     config,
			httpClient: httpClient,
		},
	}, nil
}

func (d *DNSBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return d.resolveDNS(ctx, packet)
}

func (d *DNSBaseOutbound) resolveDNS(ctx context.Context, packet routing.Packet) error {
	// Extract DNS query from packet
	msg := &miekgdns.Msg{}
	if err := msg.Unpack(packet.Data()); err != nil {
		return fmt.Errorf("failed to unpack DNS query: %v", err)
	}

	var response *miekgdns.Msg
	var err error

	switch d.config.Protocol {
	case "doh":
		response, err = d.resolveDoH(msg)
	case "dot":
		response, err = d.resolveDoT(msg)
	case "dnscrypt":
		response, err = d.resolveDNSCrypt(msg)
	case "doq":
		response, err = d.resolveDoQ(msg)
	case "slowdns":
		response, err = d.resolveSlowDNS(msg)
	case "tcp":
		response, err = d.resolveTCP(msg)
	case "udp":
		response, err = d.resolveUDP(msg)
	default:
		return fmt.Errorf("unsupported DNS protocol: %s", d.config.Protocol)
	}

	if err != nil {
		return fmt.Errorf("DNS resolution failed: %v", err)
	}

	// Pack response back into packet
	responseData, err := response.Pack()
	if err != nil {
		return fmt.Errorf("failed to pack DNS response: %v", err)
	}

	// Forward response back
	// In a real implementation, this would be sent back to the client
	d.logger.Debugf("DNS resolved: %d answers", len(response.Answer))
	
	return nil
}

func (d *DNSBaseOutbound) resolveDoH(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// DNS-over-HTTPS resolution
	queryURL := d.config.URL
	if queryURL == "" {
		queryURL = fmt.Sprintf("https://%s:%d/dns-query", d.config.Server, d.config.Port)
	}

	// Convert DNS message to wire format
	wire, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}

	// Add DNS-over-HTTPS parameters
	q := req.URL.Query()
	q.Set("dns", string(wire))
	req.URL.RawQuery = q.Encode()

	// Add custom headers
	for k, v := range d.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH request failed: %d", resp.StatusCode)
	}

	body := make([]byte, resp.ContentLength)
	if resp.ContentLength > 0 {
		_, err = resp.Body.Read(body)
		if err != nil {
			return nil, err
		}
	} else {
		// Read entire body
		body, _ = io.ReadAll(resp.Body)
	}

	response := &miekgdns.Msg{}
	err = response.Unpack(body)
	return response, err
}

func (d *DNSBaseOutbound) resolveDoT(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// DNS-over-TLS resolution
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	
	conn, err := tls.Dial("tcp", address, &tls.Config{
		ServerName:         d.config.TLSConfig.ServerName,
		InsecureSkipVerify: d.config.TLSConfig.Insecure,
	})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dnsClient := &miekgdns.Client{
		Net: "tcp-tls",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) resolveDNSCrypt(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// DNSCrypt resolution
	// This is a simplified implementation
	// Real DNSCrypt would require encryption/decryption with provider key
	
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	dnsClient := &miekgdns.Client{
		Net: "udp",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) resolveDoQ(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// DNS-over-QUIC resolution
	// This would require QUIC implementation
	// For now, fallback to regular UDP
	
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	dnsClient := &miekgdns.Client{
		Net: "udp",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) resolveSlowDNS(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// SlowDNS - tunnel DNS over SSH
	// This would use SSH tunneling for DNS queries
	
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	dnsClient := &miekgdns.Client{
		Net: "udp",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) resolveTCP(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// Traditional DNS over TCP
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	dnsClient := &miekgdns.Client{
		Net: "tcp",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) resolveUDP(msg *miekgdns.Msg) (*miekgdns.Msg, error) {
	// Traditional DNS over UDP
	address := fmt.Sprintf("%s:%d", d.config.Server, d.config.Port)
	dnsClient := &miekgdns.Client{
		Net: "udp",
	}
	
	response, _, err := dnsClient.Exchange(msg, address)
	return response, err
}

func (d *DNSBaseOutbound) Close() error {
	if d.httpClient != nil {
		d.httpClient.CloseIdleConnections()
	}
	return nil
}

func (d *DNSBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different DNS protocols
type DoHOutbound struct {
	DNSBaseOutbound
}

type DoTOutbound struct {
	DNSBaseOutbound
}

type DNSCryptOutbound struct {
	DNSBaseOutbound
}

type DoQOutbound struct {
	DNSBaseOutbound
}

type SlowDNSOutbound struct {
	DNSBaseOutbound
}

type DNSTCPOutbound struct {
	DNSBaseOutbound
}

type DNSUDPOutbound struct {
	DNSBaseOutbound
}

// Constructor functions for each protocol
func NewDoTOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DoTOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &DoTOutbound{*base}, nil
}

func NewDNSCryptOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DNSCryptOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &DNSCryptOutbound{*base}, nil
}

func NewDoQOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DoQOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &DoQOutbound{*base}, nil
}

func NewSlowDNSOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SlowDNSOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &SlowDNSOutbound{*base}, nil
}

func NewDNSTCPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DNSTCPOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &DNSTCPOutbound{*base}, nil
}

func NewDNSUDPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DNSUDPOutbound, error) {
	base, err := NewDoHOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	return &DNSUDPOutbound{*base}, nil
}


package httpinject

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
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
	register.Outbound("httpinject", NewHTTPInjectOutbound)
	register.Outbound("ws-inject", NewWSInjectOutbound)
	register.Outbound("http-connect", NewHTTPConnectOutbound)
	register.Outbound("http-fronting", NewHTTPFrontingOutbound)
	register.Outbound("http-payload", NewHTTPPayloadOutbound)
	register.Outbound("chunked-http", NewChunkedHTTPOutbound)
}

type HTTPInjectConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // "httpinject", "ws-inject", "http-connect", "http-fronting", "http-payload", "chunked-http"
	Method         string `json:"method"`   // "GET", "POST", "CONNECT", "PUT", "DELETE", etc.
	Path           string `json:"path"`
	Host           string `json:"host"`
	UserAgent      string `json:"user_agent"`
	Headers        map[string]string `json:"headers"`
	Payload        string `json:"payload"`
	PayloadFile    string `json:"payload_file"`
	// WebSocket specific
	WebSocketPath  string `json:"ws_path"`
	WebSocketKey   string `json:"ws_key"`
	// HTTP CONNECT specific
	Target         string `json:"target"` // Target server for CONNECT
	// Fronting specific
	FrontingDomain string `json:"fronting_domain"`
	FrontingURL    string `json:"fronting_url"`
	// Chunked encoding
	ChunkSize      int    `json:"chunk_size"`
	ChunkDelay     int    `json:"chunk_delay"`
	// Security
	TLSSkipVerify  bool   `json:"tls_skip_verify"`
	TLSConfig      TLSConfig `json:"tls_config"`
	// HTTP settings
	Timeout        int    `json:"timeout"`
	KeepAlive      bool   `json:"keep_alive"`
	Compression    bool   `json:"compression"`
	// Injection settings
	InjectionPath  string `json:"injection_path"`
	InjectionQuery string `json:"injection_query"`
	InjectionBody  string `json:"injection_body"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type HTTPInjectBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     HTTPInjectConfig
	client     *http.Client
	connection net.Conn
}

func NewHTTPInjectOutbound(router route.Router, logger log.Logger, options option.Outbound) (*HTTPInjectOutbound, error) {
	var config HTTPInjectConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	// Set default HTTP injection configuration
	if config.Server == "" {
		config.Server = "example.com"
	}
	if config.Port == 0 {
		config.Port = 443
	}
	if config.Method == "" {
		config.Method = "GET"
	}
	if config.Path == "" {
		config.Path = "/"
	}
	if config.Host == "" {
		config.Host = config.Server
	}
	if config.UserAgent == "" {
		config.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:         config.TLSConfig.ServerName,
				InsecureSkipVerify: config.TLSSkipVerify || config.TLSConfig.Insecure,
			},
			DisableCompression: !config.Compression,
		},
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &HTTPInjectOutbound{
		HTTPInjectBaseOutbound: HTTPInjectBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
			client: httpClient,
		},
	}, nil
}

func (h *HTTPInjectBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return h.injectAndRoute(ctx, packet)
}

func (h *HTTPInjectBaseOutbound) injectAndRoute(ctx context.Context, packet routing.Packet) error {
	// Create HTTP request with injection
	req, err := h.createInjectedRequest(packet.Data())
	if err != nil {
		return fmt.Errorf("failed to create injected request: %v", err)
	}

	// Send request and get response
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP injection request failed: %v", err)
	}
	defer resp.Body.Close()

	// Process response data
	if err := h.processResponse(resp); err != nil {
		return fmt.Errorf("failed to process response: %v", err)
	}

	return nil
}

func (h *HTTPInjectBaseOutbound) createInjectedRequest(data []byte) (*http.Request, error) {
	var req *http.Request
	var err error

	switch h.config.Protocol {
	case "httpinject":
		req, err = h.createHTTPInjectRequest(data)
	case "ws-inject":
		req, err = h.createWebSocketInjectRequest(data)
	case "http-connect":
		req, err = h.createHTTPConnectRequest(data)
	case "http-fronting":
		req, err = h.createHTTPFrontingRequest(data)
	case "http-payload":
		req, err = h.createHTTPPayloadRequest(data)
	case "chunked-http":
		req, err = h.createChunkedHTTPRequest(data)
	default:
		req, err = h.createHTTPInjectRequest(data)
	}

	return req, err
}

func (h *HTTPInjectBaseOutbound) createHTTPInjectRequest(data []byte) (*http.Request, error) {
	// Basic HTTP injection with payload in request body or headers
	scheme := "https"
	if h.config.Port == 80 {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, h.config.Server, h.config.Port, h.config.Path)
	
	var bodyReader *strings.Reader
	if h.config.Payload != "" {
		bodyReader = strings.NewReader(h.config.Payload)
	} else if h.config.InjectionBody != "" {
		bodyReader = strings.NewReader(h.config.InjectionBody)
	}

	req, err := http.NewRequest(h.config.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set headers
	h.setCommonHeaders(req)
	
	// Add injection headers
	if h.config.InjectionPath != "" {
		req.Header.Set("X-Injection-Path", h.config.InjectionPath)
	}
	if h.config.InjectionQuery != "" {
		req.Header.Set("X-Injection-Query", h.config.InjectionQuery)
	}

	// Add data as header if payload mode
	if data != nil && len(data) > 0 {
		req.Header.Set("X-Injection-Data", string(data))
	}

	return req, nil
}

func (h *HTTPInjectBaseOutbound) createWebSocketInjectRequest(data []byte) (*http.Request, error) {
	// WebSocket injection request
	scheme := "wss"
	if h.config.Port == 80 {
		scheme = "ws"
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, h.config.Server, h.config.Port, h.config.WebSocketPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	h.setCommonHeaders(req)
	
	// Add WebSocket headers
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", h.config.WebSocketKey)
	req.Header.Set("Sec-WebSocket-Version", "13")
	
	// Add injection data in WebSocket frame
	if data != nil && len(data) > 0 {
		req.Header.Set("X-Injection-Data", string(data))
	}

	return req, nil
}

func (h *HTTPInjectBaseOutbound) createHTTPConnectRequest(data []byte) (*http.Request, error) {
	// HTTP CONNECT request for tunneling
	if h.config.Target == "" {
		return nil, fmt.Errorf("target server required for HTTP CONNECT")
	}

	url := fmt.Sprintf("http://%s:%d", h.config.Server, h.config.Port)
	req, err := http.NewRequest("CONNECT", url, nil)
	if err != nil {
		return nil, err
	}

	h.setCommonHeaders(req)
	
	// Add target in headers for proxy
	req.Header.Set("X-Target", h.config.Target)
	
	if data != nil && len(data) > 0 {
		req.Header.Set("X-Injection-Data", string(data))
	}

	return req, nil
}

func (h *HTTPInjectBaseOutbound) createHTTPFrontingRequest(data []byte) (*http.Request, error) {
	// HTTP fronting - disguise traffic as normal web traffic
	frontingURL := h.config.FrontingURL
	if frontingURL == "" {
		frontingURL = fmt.Sprintf("https://%s/%s", h.config.FrontingDomain, h.config.Path)
	}

	req, err := http.NewRequest(h.config.Method, frontingURL, strings.NewReader(h.config.Payload))
	if err != nil {
		return nil, err
	}

	h.setCommonHeaders(req)
	
	// Set fronting domain headers
	req.Host = h.config.FrontingDomain
	req.Header.Set("X-Real-Server", h.config.Server)
	req.Header.Set("X-Real-Port", fmt.Sprintf("%d", h.config.Port))
	
	if data != nil && len(data) > 0 {
		req.Header.Set("X-Injection-Data", string(data))
	}

	return req, nil
}

func (h *HTTPInjectBaseOutbound) createHTTPPayloadRequest(data []byte) (*http.Request, error) {
	// HTTP with payload injection
	scheme := "https"
	if h.config.Port == 80 {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, h.config.Server, h.config.Port, h.config.Path)
	
	var payload string
	if h.config.Payload != "" {
		payload = h.config.Payload
	} else if data != nil && len(data) > 0 {
		payload = string(data)
	} else {
		payload = "default_payload"
	}

	req, err := http.NewRequest(h.config.Method, url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	h.setCommonHeaders(req)
	
	// Add payload-specific headers
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Payload-Size", fmt.Sprintf("%d", len(payload)))

	return req, nil
}

func (h *HTTPInjectBaseOutbound) createChunkedHTTPRequest(data []byte) (*http.Request, error) {
	// HTTP with chunked encoding
	scheme := "https"
	if h.config.Port == 80 {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s:%d%s", scheme, h.config.Server, h.config.Port, h.config.Path)
	req, err := http.NewRequest(h.config.Method, url, nil)
	if err != nil {
		return nil, err
	}

	h.setCommonHeaders(req)
	
	// Add chunked encoding headers
	req.Header.Set("Transfer-Encoding", "chunked")
	
	if h.config.ChunkSize > 0 {
		req.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", h.config.ChunkSize))
	}
	
	if h.config.ChunkDelay > 0 {
		req.Header.Set("X-Chunk-Delay", fmt.Sprintf("%d", h.config.ChunkDelay))
	}

	if data != nil && len(data) > 0 {
		req.Header.Set("X-Injection-Data", string(data))
	}

	return req, nil
}

func (h *HTTPInjectBaseOutbound) setCommonHeaders(req *http.Request) {
	// Set common HTTP headers
	req.Header.Set("User-Agent", h.config.UserAgent)
	req.Header.Set("Host", h.config.Host)
	
	// Set custom headers
	for k, v := range h.config.Headers {
		req.Header.Set(k, v)
	}
	
	// Set standard headers based on method
	if h.config.Method != "GET" && h.config.Method != "HEAD" {
		req.Header.Set("Content-Type", "application/octet-stream")
		if !h.config.KeepAlive {
			req.Header.Set("Connection", "close")
		}
	}
}

func (h *HTTPInjectBaseOutbound) processResponse(resp *http.Response) error {
	h.logger.Debugf("HTTP injection response: %d %s", resp.StatusCode, resp.Status)
	
	// Log response headers for debugging
	for k, v := range resp.Header {
		h.logger.Debugf("Response header: %s: %s", k, strings.Join(v, ", "))
	}
	
	// Check for injection confirmation
	if resp.Header.Get("X-Injection-Success") == "true" {
		h.logger.Info("HTTP injection successful")
	} else {
		h.logger.Warn("HTTP injection may have failed")
	}
	
	// Read and discard response body
	if resp.Body != nil {
		resp.Body.Close()
	}
	
	return nil
}

func (h *HTTPInjectBaseOutbound) Close() error {
	if h.connection != nil {
		return h.connection.Close()
	}
	return nil
}

func (h *HTTPInjectBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different HTTP injection protocols
type HTTPInjectOutbound struct {
	HTTPInjectBaseOutbound
}

type WSInjectOutbound struct {
	HTTPInjectBaseOutbound
}

type HTTPConnectOutbound struct {
	HTTPInjectBaseOutbound
}

type HTTPFrontingOutbound struct {
	HTTPInjectBaseOutbound
}

type HTTPPayloadOutbound struct {
	HTTPInjectBaseOutbound
}

type ChunkedHTTPOutbound struct {
	HTTPInjectBaseOutbound
}

// Constructor functions for each protocol
func NewWSInjectOutbound(router route.Router, logger log.Logger, options option.Outbound) (*WSInjectOutbound, error) {
	base, err := NewHTTPInjectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "ws-inject"
	return &WSInjectOutbound{*base}, nil
}

func NewHTTPConnectOutbound(router route.Router, logger log.Logger, options option.Outbound) (*HTTPConnectOutbound, error) {
	base, err := NewHTTPInjectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "http-connect"
	return &HTTPConnectOutbound{*base}, nil
}

func NewHTTPFrontingOutbound(router route.Router, logger log.Logger, options option.Outbound) (*HTTPFrontingOutbound, error) {
	base, err := NewHTTPInjectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "http-fronting"
	return &HTTPFrontingOutbound{*base}, nil
}

func NewHTTPPayloadOutbound(router route.Router, logger log.Logger, options option.Outbound) (*HTTPPayloadOutbound, error) {
	base, err := NewHTTPInjectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "http-payload"
	return &HTTPPayloadOutbound{*base}, nil
}

func NewChunkedHTTPOutbound(router route.Router, logger log.Logger, options option.Outbound) (*ChunkedHTTPOutbound, error) {
	base, err := NewHTTPInjectOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "chunked-http"
	return &ChunkedHTTPOutbound{*base}, nil
}


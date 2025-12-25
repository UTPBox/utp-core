package stealth

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
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
	register.Outbound("steganographic", NewSteganographicOutbound)
	register.Outbound("icmp-tunnel", NewICMPTunnelOutbound)
	register.Outbound("dns-tunnel", NewDNSTunnelOutbound)
	register.Outbound("email-tunnel", NewEmailTunnelOutbound)
	register.Outbound("image-steganography", NewImageSteganographyOutbound)
	register.Outbound("audio-steganography", NewAudioSteganographyOutbound)
	register.Outbound("carrier-steganography", NewCarrierSteganographyOutbound)
}

type StealthConfig struct {
	Server         string `json:"server"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"` // "steganographic", "icmp-tunnel", "dns-tunnel", "email-tunnel", "image-steganography", "audio-steganography", "carrier-steganography"
	Method         string `json:"method"`   // "lsb", "dct", "dwt", "spread-spectrum"
	Carrier        string `json:"carrier"`  // Carrier file for steganography
	Key            string `json:"key"`      // Encryption key
	StegoAlgorithm string `json:"stego_algorithm"` // "lsb", "dct", "spread-spectrum"
	// Image steganography
	ImageFormat    string `json:"image_format"` // "png", "jpg", "bmp"
	ImagePath      string `json:"image_path"`
	// Audio steganography
	AudioFormat    string `json:"audio_format"` // "wav", "mp3", "flac"
	AudioPath      string `json:"audio_path"`
	// Email tunnel
	SMTPServer     string `json:"smtp_server"`
	SMTPPort       int    `json:"smtp_port"`
	SMTPUser       string `json:"smtp_user"`
	SMTPPassword   string `json:"smtp_password"`
	EmailFrom      string `json:"email_from"`
	EmailTo        string `json:"email_to"`
	Subject        string `json:"subject"`
	// DNS tunnel
	DNSOverHTTPS   string `json:"dns_over_https"` // DoH server
	DNSServer      string `json:"dns_server"`     // DNS server
	DNSDomain      string `json:"dns_domain"`     // Domain for tunneling
	// ICMP tunnel
	ICMPType       int    `json:"icmp_type"`      // ICMP type (8 for echo request)
	// Carrier file
	CarrierPath    string `json:"carrier_path"`
	CarrierFormat  string `json:"carrier_format"`
	// Encoding
	Encoding       string `json:"encoding"`       // "base64", "hex", "url"
	// Security
	Encryption     bool   `json:"encryption"`
	TLSConfig      TLSConfig `json:"tls_config"`
	Timeout        int    `json:"timeout"`
}

type TLSConfig struct {
	ServerName string `json:"server_name"`
	CA         string `json:"ca"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
	Insecure   bool   `json:"insecure"`
	NextProto  string `json:"next_proto"`
}

type StealthBaseOutbound struct {
	ctx        context.Context
	logger     log.Logger
	router     route.Router
	config     StealthConfig
	connection net.Conn
}

func NewSteganographicOutbound(router route.Router, logger log.Logger, options option.Outbound) (*SteganographicOutbound, error) {
	var config StealthConfig
	if options.Options != nil {
		if err := common.Decode(options.Options, &config); err != nil {
			return nil, err
		}
	}

	// Set default stealth configuration
	if config.Server == "" {
		config.Server = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.Protocol == "" {
		config.Protocol = "steganographic"
	}
	if config.Method == "" {
		config.Method = "lsb"
	}
	if config.Encoding == "" {
		config.Encoding = "base64"
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	return &SteganographicOutbound{
		StealthBaseOutbound: StealthBaseOutbound{
			ctx:    context.Background(),
			logger: logger,
			router: router,
			config: config,
		},
	}, nil
}

func (s *StealthBaseOutbound) Route(ctx context.Context, packet routing.Packet) error {
	return s.stealthAndRoute(ctx, packet)
}

func (s *StealthBaseOutbound) stealthAndRoute(ctx context.Context, packet routing.Packet) error {
	// Apply steganographic method to hide data
	steganographedData, err := s.applySteganography(packet.Data())
	if err != nil {
		return fmt.Errorf("steganography failed: %v", err)
	}

	// Route through appropriate covert channel
	_, err = s.sendSteganographedData(steganographedData)
	return err
}

func (s *StealthBaseOutbound) applySteganography(data []byte) ([]byte, error) {
	switch s.config.Protocol {
	case "steganographic":
		return s.applyImageSteganography(data)
	case "image-steganography":
		return s.applyImageSteganography(data)
	case "audio-steganography":
		return s.applyAudioSteganography(data)
	case "email-tunnel":
		return s.applyEmailSteganography(data)
	case "dns-tunnel":
		return s.applyDNSSteganography(data)
	case "icmp-tunnel":
		return s.applyICMPSteganography(data)
	case "carrier-steganography":
		return s.applyCarrierSteganography(data)
	default:
		return data, nil
	}
}

func (s *StealthBaseOutbound) applyImageSteganography(data []byte) ([]byte, error) {
	// Apply image steganography using LSB (Least Significant Bit) method
	s.logger.Debug("Applying image steganography")
	
	// Load carrier image
	if s.config.CarrierPath == "" {
		s.config.CarrierPath = "carrier.png"
	}
	
	// Encode data
	var encodedData []byte
	switch s.config.Encoding {
	case "base64":
		encodedData = []byte(base64.StdEncoding.EncodeToString(data))
	case "hex":
		encodedData = []byte(strings.ToUpper(hex.EncodeToString(data)))
	case "url":
		encodedData = []byte(url.QueryEscape(string(data)))
	default:
		encodedData = data
	}
	
	// Add delimiter and metadata
	stegoData := s.addStegoMetadata(encodedData)
	
	// Embed in carrier image (simplified LSB implementation)
	return s.embedInImageLSB(stegoData), nil
}

func (s *StealthBaseOutbound) applyAudioSteganography(data []byte) ([]byte, error) {
	// Apply audio steganography using spread spectrum method
	s.logger.Debug("Applying audio steganography")
	
	// Encode data
	var encodedData []byte
	switch s.config.Encoding {
	case "base64":
		encodedData = []byte(base64.StdEncoding.EncodeToString(data))
	default:
		encodedData = data
	}
	
	// Add metadata and embed in audio
	stegoData := s.addStegoMetadata(encodedData)
	return s.embedInAudioSpreadSpectrum(stegoData), nil
}

func (s *StealthBaseOutbound) applyEmailSteganography(data []byte) ([]byte, error) {
	// Encode data for email transmission
	s.logger.Debug("Applying email steganography")
	
	// Encode data
	var encodedData []byte
	switch s.config.Encoding {
	case "base64":
		encodedData = []byte(base64.StdEncoding.EncodeToString(data))
	default:
		encodedData = data
	}
	
	// Add metadata
	stegoData := s.addStegoMetadata(encodedData)
	
	// Send via email (simplified)
	err := s.sendEmailWithData(stegoData)
	return nil, err
}

func (s *StealthBaseOutbound) applyDNSSteganography(data []byte) ([]byte, error) {
	// Encode data for DNS tunneling
	s.logger.Debug("Applying DNS steganography")
	
	// Split data into DNS queries
	return s.encodeAsDNSQueries(data), nil
}

func (s *StealthBaseOutbound) applyICMPSteganography(data []byte) ([]byte, error) {
	// Encode data for ICMP tunneling
	s.logger.Debug("Applying ICMP steganography")
	
	// Split data into ICMP packets
	return s.encodeAsICMPPackets(data), nil
}

func (s *StealthBaseOutbound) applyCarrierSteganography(data []byte) ([]byte, error) {
	// Use carrier file for steganography
	s.logger.Debug("Applying carrier steganography")
	
	if s.config.CarrierPath == "" {
		return nil, fmt.Errorf("carrier file path required")
	}
	
	// Read carrier file
	carrierData, err := os.ReadFile(s.config.CarrierPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read carrier file: %v", err)
	}
	
	// Embed data in carrier
	return s.embedInCarrier(carrierData, data), nil
}

func (s *StealthBaseOutbound) sendSteganographedData(data []byte) (int, error) {
	switch s.config.Protocol {
	case "email-tunnel":
		return 0, s.sendEmailWithData(data)
	case "dns-tunnel":
		return s.sendDNSData(data)
	case "icmp-tunnel":
		return s.sendICMPData(data)
	case "image-steganography", "audio-steganography", "steganographic", "carrier-steganography":
		return s.sendCarrierData(data)
	default:
		return 0, fmt.Errorf("unsupported stealth protocol: %s", s.config.Protocol)
	}
}

func (s *StealthBaseOutbound) addStegoMetadata(data []byte) []byte {
	// Add metadata to steganographed data
	// Format: [Magic] [Length] [Data] [Checksum]
	
	magic := []byte("STEGO")
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	
	// Simple checksum
	var checksum uint32
	for _, b := range data {
		checksum += uint32(b)
	}
	
	checksumBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(checksumBytes, checksum)
	
	result := append(magic, length...)
	result = append(result, data...)
	result = append(result, checksumBytes...)
	
	return result
}

func (s *StealthBaseOutbound) embedInImageLSB(data []byte) []byte {
	// Simplified LSB steganography implementation
	// In reality, this would manipulate image pixels
	
	// For demonstration, we'll just return the data
	// Real implementation would embed in image pixels
	return data
}

func (s *StealthBaseOutbound) embedInAudioSpreadSpectrum(data []byte) []byte {
	// Simplified spread spectrum steganography
	// In reality, this would modify audio frequency domain
	
	return data
}

func (s *StealthBaseOutbound) embedInCarrier(carrierData []byte, payloadData []byte) []byte {
	// Embed payload in carrier file
	// Simplified implementation
	
	if len(payloadData) > len(carrierData)/8 {
		// Payload too large for carrier
		return carrierData
	}
	
	// Simple embedding: XOR with carrier data
	result := make([]byte, len(carrierData))
	copy(result, carrierData)
	
	for i, b := range payloadData {
		if i < len(result) {
			result[i] ^= b
		}
	}
	
	return result
}

func (s *StealthBaseOutbound) encodeAsDNSQueries(data []byte) []byte {
	// Encode data as DNS queries
	// Each query encodes a portion of the data
	
	var queries []string
	for i := 0; i < len(data); i += 10 {
		end := i + 10
		if end > len(data) {
			end = len(data)
		}
		
		chunk := data[i:end]
		encoded := base64.StdEncoding.EncodeToString(chunk)
		query := fmt.Sprintf("%s.%s", encoded, s.config.DNSDomain)
		queries = append(queries, query)
	}
	
	return []byte(strings.Join(queries, ","))
}

func (s *StealthBaseOutbound) encodeAsICMPPackets(data []byte) []byte {
	// Encode data as ICMP packets
	var packets []string
	
	for i := 0; i < len(data); i += 100 {
		end := i + 100
		if end > len(data) {
			end = len(data)
		}
		
		chunk := data[i:end]
		encoded := base64.StdEncoding.EncodeToString(chunk)
		packets = append(packets, encoded)
	}
	
	return []byte(strings.Join(packets, ","))
}

func (s *StealthBaseOutbound) sendEmailWithData(data []byte) error {
	s.logger.Info("Sending steganographed data via email")
	
	// Simplified email sending
	// In reality, would use proper SMTP library
	
	if s.config.SMTPServer == "" {
		s.config.SMTPServer = "smtp.gmail.com"
	}
	if s.config.SMTPPort == 0 {
		s.config.SMTPPort = 587
	}
	
	// Create email with encoded data
	subject := s.config.Subject
	if subject == "" {
		subject = "Steganographed Data Transfer"
	}
	
	body := base64.StdEncoding.EncodeToString(data)
	
	s.logger.Debugf("Email would be sent to: %s", s.config.EmailTo)
	s.logger.Debugf("Subject: %s", subject)
	s.logger.Debugf("Body length: %d bytes", len(body))
	
	return nil
}

func (s *StealthBaseOutbound) sendDNSData(data []byte) (int, error) {
	s.logger.Info("Sending steganographed data via DNS")
	
	// Simplified DNS sending
	queries := s.encodeAsDNSQueries(data)
	
	s.logger.Debugf("DNS queries: %s", string(queries))
	
	return len(data), nil
}

func (s *StealthBaseOutbound) sendICMPData(data []byte) (int, error) {
	s.logger.Info("Sending steganographed data via ICMP")
	
	// Simplified ICMP sending
	packets := s.encodeAsICMPPackets(data)
	
	s.logger.Debugf("ICMP packets: %s", string(packets))
	
	return len(data), nil
}

func (s *StealthBaseOutbound) sendCarrierData(data []byte) (int, error) {
	s.logger.Info("Sending steganographed data via carrier file")
	
	// Read carrier file and embed data
	carrierData, err := os.ReadFile(s.config.CarrierPath)
	if err != nil {
		return 0, err
	}
	
	stegoData := s.embedInCarrier(carrierData, data)
	
	// In real implementation, would send via appropriate channel
	s.logger.Debugf("Carrier steganography completed, size: %d bytes", len(stegoData))
	
	return len(data), nil
}

func (s *StealthBaseOutbound) Close() error {
	if s.connection != nil {
		return s.connection.Close()
	}
	return nil
}

func (s *StealthBaseOutbound) Stack() transport.Stack {
	return transport.Stack{}
}

// Concrete types for different stealth protocols
type SteganographicOutbound struct {
	StealthBaseOutbound
}

type ICMPTunnelOutbound struct {
	StealthBaseOutbound
}

type DNSTunnelOutbound struct {
	StealthBaseOutbound
}

type EmailTunnelOutbound struct {
	StealthBaseOutbound
}

type ImageSteganographyOutbound struct {
	StealthBaseOutbound
}

type AudioSteganographyOutbound struct {
	StealthBaseOutbound
}

type CarrierSteganographyOutbound struct {
	StealthBaseOutbound
}

// Constructor functions for each stealth protocol
func NewICMPTunnelOutbound(router route.Router, logger log.Logger, options option.Outbound) (*ICMPTunnelOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "icmp-tunnel"
	return &ICMPTunnelOutbound{*base}, nil
}

func NewDNSTunnelOutbound(router route.Router, logger log.Logger, options option.Outbound) (*DNSTunnelOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "dns-tunnel"
	return &DNSTunnelOutbound{*base}, nil
}

func NewEmailTunnelOutbound(router route.Router, logger log.Logger, options option.Outbound) (*EmailTunnelOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "email-tunnel"
	return &EmailTunnelOutbound{*base}, nil
}

func NewImageSteganographyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*ImageSteganographyOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "image-steganography"
	return &ImageSteganographyOutbound{*base}, nil
}

func NewAudioSteganographyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*AudioSteganographyOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "audio-steganography"
	return &AudioSteganographyOutbound{*base}, nil
}

func NewCarrierSteganographyOutbound(router route.Router, logger log.Logger, options option.Outbound) (*CarrierSteganographyOutbound, error) {
	base, err := NewSteganographicOutbound(router, logger, options)
	if err != nil {
		return nil, err
	}
	
	base.config.Protocol = "carrier-steganography"
	return &CarrierSteganographyOutbound{*base}, nil
}


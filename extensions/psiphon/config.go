package psiphon

// PsiphonOptions defines the configuration for the Psiphon outbound protocol
type PsiphonOptions struct {
	Server     string `json:"server"`      // Server hostname or IP
	Port       int    `json:"port"`        // Server port
	Username   string `json:"username"`    // SSH Username
	Password   string `json:"password"`    // SSH Password
	UseTLS     bool   `json:"use_tls"`     // Enable TLS wrapping
	HeaderHost string `json:"header_host"` // Optional HTTP Host header
	Obfuscate  bool   `json:"obfuscate"`   // Enable additional obfuscation (placeholder)
}

package psiphon

import (
	"bytes"
	"fmt"
	"net"
)

// doHTTPHandshake performs a Psiphon-style HTTP handshake
func doHTTPHandshake(conn net.Conn, opts PsiphonOptions) error {
	host := opts.HeaderHost
	if host == "" {
		host = opts.Server // Fallback to server address if no host header provided
	}

	// Construct HTTP CONNECT request
	// Note: Psiphon often uses specific variations, this is a standard implementation
	req := fmt.Sprintf("CONNECT %s:%d HTTP/1.1\r\nHost: %s\r\n\r\n",
		opts.Server, opts.Port, host)

	// Write request
	_, err := conn.Write([]byte(req))
	if err != nil {
		return fmt.Errorf("failed to write HTTP handshake: %w", err)
	}

	// Read response (expecting HTTP 200)
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read HTTP handshake response: %w", err)
	}

	// Check for success (200 OK)
	// We look for "200" to be flexible with the exact status line text
	if !bytes.Contains(buf[:n], []byte("200")) {
		return fmt.Errorf("HTTP handshake failed, response: %s", string(buf[:n]))
	}

	return nil
}

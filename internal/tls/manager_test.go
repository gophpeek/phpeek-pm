package tls

import (
	"crypto/tls"
	"log/slog"
	"os"
	"testing"

	"github.com/gophpeek/phpeek-pm/internal/config"
)

// TestNewManager_Disabled tests that disabled TLS returns error
func TestNewManager_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.TLSConfig{
		Enabled: false,
	}

	_, err := NewManager(cfg, logger)
	if err == nil {
		t.Error("Expected error when TLS is disabled, got nil")
	}
}

// TestNewManager_MissingFiles tests that missing cert/key returns error
func TestNewManager_MissingFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.TLSConfig{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}

	_, err := NewManager(cfg, logger)
	if err == nil {
		t.Error("Expected error when cert/key files don't exist, got nil")
	}
}

// TestParseTLSVersion tests TLS version parsing
func TestParseTLSVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	m := &Manager{
		config: &config.TLSConfig{},
		logger: logger,
	}

	tests := []struct {
		version  string
		expected uint16
	}{
		{"TLS 1.0", tls.VersionTLS10},
		{"TLS 1.1", tls.VersionTLS11},
		{"TLS 1.2", tls.VersionTLS12},
		{"TLS 1.3", tls.VersionTLS13},
		{"invalid", tls.VersionTLS12}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := m.parseTLSVersion(tt.version)
			if got != tt.expected {
				t.Errorf("parseTLSVersion(%s) = %v, want %v", tt.version, got, tt.expected)
			}
		})
	}
}

// TestParseClientAuth tests client auth parsing
func TestParseClientAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	m := &Manager{
		config: &config.TLSConfig{},
		logger: logger,
	}

	tests := []struct {
		auth     string
		expected tls.ClientAuthType
	}{
		{"none", tls.NoClientCert},
		{"request", tls.RequestClientCert},
		{"require", tls.RequireAnyClientCert},
		{"verify", tls.RequireAndVerifyClientCert},
		{"invalid", tls.NoClientCert}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.auth, func(t *testing.T) {
			got := m.parseClientAuth(tt.auth)
			if got != tt.expected {
				t.Errorf("parseClientAuth(%s) = %v, want %v", tt.auth, got, tt.expected)
			}
		})
	}
}

// TestParseCipherSuites tests cipher suite parsing
func TestParseCipherSuites(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	m := &Manager{
		config: &config.TLSConfig{},
		logger: logger,
	}

	tests := []struct {
		name        string
		suites      []string
		shouldError bool
	}{
		{
			name:        "valid TLS 1.2 suites",
			suites:      []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			shouldError: false,
		},
		{
			name:        "valid TLS 1.3 suites",
			suites:      []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
			shouldError: false,
		},
		{
			name:        "invalid suite",
			suites:      []string{"TLS_INVALID_CIPHER"},
			shouldError: true,
		},
		{
			name:        "empty",
			suites:      []string{},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphers, err := m.parseCipherSuites(tt.suites)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error for invalid cipher suite, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(ciphers) != len(tt.suites) {
					t.Errorf("Expected %d ciphers, got %d", len(tt.suites), len(ciphers))
				}
			}
		})
	}
}

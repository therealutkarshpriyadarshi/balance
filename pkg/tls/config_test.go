package tls

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MinVersion != VersionTLS12 {
		t.Errorf("Expected MinVersion to be TLS 1.2, got %d", cfg.MinVersion)
	}

	if cfg.MaxVersion != VersionTLS13 {
		t.Errorf("Expected MaxVersion to be TLS 1.3, got %d", cfg.MaxVersion)
	}

	if !cfg.PreferServerCipherSuites {
		t.Error("Expected PreferServerCipherSuites to be true")
	}

	if len(cfg.NextProtos) != 2 {
		t.Errorf("Expected 2 NextProtos, got %d", len(cfg.NextProtos))
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected TLSVersion
		wantErr  bool
	}{
		{"1.0", VersionTLS10, false},
		{"1.1", VersionTLS11, false},
		{"1.2", VersionTLS12, false},
		{"1.3", VersionTLS13, false},
		{"1.4", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			version, err := ParseTLSVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTLSVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if version != tt.expected {
				t.Errorf("ParseTLSVersion() = %v, want %v", version, tt.expected)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid min version",
			config: &Config{
				MinVersion: 0,
			},
			wantErr: true,
		},
		{
			name: "min > max",
			config: &Config{
				MinVersion: VersionTLS13,
				MaxVersion: VersionTLS12,
			},
			wantErr: true,
		},
		{
			name: "valid TLS 1.2+",
			config: &Config{
				MinVersion: VersionTLS12,
				MaxVersion: VersionTLS13,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	clone := original.Clone()

	// Modify clone
	clone.MinVersion = VersionTLS10
	clone.NextProtos[0] = "modified"

	// Check that original is unchanged
	if original.MinVersion == clone.MinVersion {
		t.Error("Clone is not independent - MinVersion was modified")
	}

	if original.NextProtos[0] == "modified" {
		t.Error("Clone is not independent - NextProtos was modified")
	}
}

func TestToStdConfig(t *testing.T) {
	cfg := DefaultConfig()
	stdCfg := cfg.ToStdConfig()

	if stdCfg.MinVersion != uint16(cfg.MinVersion) {
		t.Errorf("MinVersion mismatch: got %d, want %d", stdCfg.MinVersion, cfg.MinVersion)
	}

	if stdCfg.MaxVersion != uint16(cfg.MaxVersion) {
		t.Errorf("MaxVersion mismatch: got %d, want %d", stdCfg.MaxVersion, cfg.MaxVersion)
	}

	if stdCfg.PreferServerCipherSuites != cfg.PreferServerCipherSuites {
		t.Error("PreferServerCipherSuites mismatch")
	}
}

func TestSecureCipherSuites(t *testing.T) {
	suites := SecureCipherSuites()

	if len(suites) == 0 {
		t.Error("Expected at least one cipher suite")
	}

	// Check that all returned suites are valid
	// This is a basic check - we're not validating the specific cipher suites
	for _, suite := range suites {
		if suite == 0 {
			t.Error("Invalid cipher suite: 0")
		}
	}
}

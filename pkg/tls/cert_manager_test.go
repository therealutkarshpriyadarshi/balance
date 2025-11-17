package tls

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestNewCertificateManager(t *testing.T) {
	cm := NewCertificateManager()

	if cm == nil {
		t.Fatal("Expected non-nil certificate manager")
	}

	if cm.certificates == nil {
		t.Fatal("Expected non-nil certificates map")
	}
}

func TestGenerateSelfSignedCertificate(t *testing.T) {
	domains := []string{"example.com", "www.example.com"}
	cert, err := GenerateSelfSignedCertificate(domains)

	if err != nil {
		t.Fatalf("Failed to generate self-signed certificate: %v", err)
	}

	if cert == nil {
		t.Fatal("Expected non-nil certificate")
	}

	if len(cert.Domains) != len(domains) {
		t.Errorf("Expected %d domains, got %d", len(domains), len(cert.Domains))
	}

	if cert.Cert == nil {
		t.Fatal("Expected non-nil X.509 certificate")
	}

	// Check validity period
	if cert.NotBefore.After(time.Now()) {
		t.Error("Certificate is not yet valid")
	}

	if cert.NotAfter.Before(time.Now()) {
		t.Error("Certificate has expired")
	}
}

func TestCertificateManagerAddCertificate(t *testing.T) {
	cm := NewCertificateManager()

	// Generate a test certificate
	cert, err := GenerateSelfSignedCertificate([]string{"example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	// Add certificate
	err = cm.AddCertificate(cert)
	if err != nil {
		t.Fatalf("Failed to add certificate: %v", err)
	}

	// Check that default cert is set
	if cm.defaultCert != cert {
		t.Error("Expected default certificate to be set")
	}
}

func TestCertificateManagerGetCertificate(t *testing.T) {
	cm := NewCertificateManager()

	// Generate and add a test certificate
	cert, err := GenerateSelfSignedCertificate([]string{"example.com", "www.example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	err = cm.AddCertificate(cert)
	if err != nil {
		t.Fatalf("Failed to add certificate: %v", err)
	}

	tests := []struct {
		name       string
		serverName string
		wantErr    bool
	}{
		{
			name:       "exact match - example.com",
			serverName: "example.com",
			wantErr:    false,
		},
		{
			name:       "exact match - www.example.com",
			serverName: "www.example.com",
			wantErr:    false,
		},
		{
			name:       "no match - fallback to default",
			serverName: "other.com",
			wantErr:    false,
		},
		{
			name:       "empty SNI - use default",
			serverName: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hello := &tls.ClientHelloInfo{
				ServerName: tt.serverName,
			}

			tlsCert, err := cm.GetCertificate(hello)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertificate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tlsCert == nil && !tt.wantErr {
				t.Error("Expected non-nil certificate")
			}
		})
	}
}

func TestCertificateManagerWildcard(t *testing.T) {
	cm := NewCertificateManager()

	// Generate wildcard certificate
	cert, err := GenerateSelfSignedCertificate([]string{"*.example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	err = cm.AddCertificate(cert)
	if err != nil {
		t.Fatalf("Failed to add certificate: %v", err)
	}

	tests := []struct {
		name       string
		serverName string
		wantErr    bool
	}{
		{
			name:       "wildcard match - www.example.com",
			serverName: "www.example.com",
			wantErr:    false,
		},
		{
			name:       "wildcard match - api.example.com",
			serverName: "api.example.com",
			wantErr:    false,
		},
		{
			name:       "no match - example.com (apex domain)",
			serverName: "example.com",
			wantErr:    false, // Falls back to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hello := &tls.ClientHelloInfo{
				ServerName: tt.serverName,
			}

			tlsCert, err := cm.GetCertificate(hello)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertificate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tlsCert == nil && !tt.wantErr {
				t.Error("Expected non-nil certificate")
			}
		})
	}
}

func TestCertificateManagerRemoveCertificate(t *testing.T) {
	cm := NewCertificateManager()

	cert, err := GenerateSelfSignedCertificate([]string{"example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	err = cm.AddCertificate(cert)
	if err != nil {
		t.Fatalf("Failed to add certificate: %v", err)
	}

	// Remove certificate
	cm.RemoveCertificate("example.com")

	// Verify it's removed (but default cert still exists)
	hello := &tls.ClientHelloInfo{ServerName: "example.com"}
	tlsCert, err := cm.GetCertificate(hello)

	// Should still return default cert
	if err != nil {
		t.Errorf("Unexpected error after removal: %v", err)
	}
	if tlsCert == nil {
		t.Error("Expected default certificate to still be available")
	}
}

func TestCertificateManagerListCertificates(t *testing.T) {
	cm := NewCertificateManager()

	// Add multiple certificates
	cert1, _ := GenerateSelfSignedCertificate([]string{"example.com"})
	cert2, _ := GenerateSelfSignedCertificate([]string{"test.com", "www.test.com"})

	cm.AddCertificate(cert1)
	cm.AddCertificate(cert2)

	certs := cm.ListCertificates()

	// Should have 2 unique certificates (cert2 is added for 2 domains but is 1 cert)
	if len(certs) != 2 {
		t.Errorf("Expected 2 certificates, got %d", len(certs))
	}
}

func TestCertificateManagerCheckExpiry(t *testing.T) {
	cm := NewCertificateManager()

	// Generate a certificate (valid for 1 year)
	cert, err := GenerateSelfSignedCertificate([]string{"example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate: %v", err)
	}

	cm.AddCertificate(cert)

	// Check for certificates expiring within 2 years (should include our 1-year cert)
	expiring := cm.CheckExpiry(2 * 365 * 24 * time.Hour)
	if len(expiring) != 1 {
		t.Errorf("Expected 1 expiring certificate, got %d", len(expiring))
	}

	// Check for certificates expiring within 1 day (should not include our 1-year cert)
	expiring = cm.CheckExpiry(24 * time.Hour)
	if len(expiring) != 0 {
		t.Errorf("Expected 0 expiring certificates, got %d", len(expiring))
	}
}

func TestValidateCertificate(t *testing.T) {
	cm := NewCertificateManager()

	// Test with nil certificate
	err := cm.validateCertificate(nil)
	if err == nil {
		t.Error("Expected error for nil certificate")
	}

	// Test with valid certificate
	cert, _ := GenerateSelfSignedCertificate([]string{"example.com"})
	err = cm.validateCertificate(cert)
	if err != nil {
		t.Errorf("Unexpected error for valid certificate: %v", err)
	}
}

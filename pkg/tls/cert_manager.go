package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"
)

// Certificate represents a TLS certificate with its private key
type Certificate struct {
	// Cert is the X.509 certificate
	Cert *x509.Certificate

	// TLSCert is the tls.Certificate for use in TLS connections
	TLSCert tls.Certificate

	// Domains is the list of domains this certificate is valid for
	Domains []string

	// NotBefore is when the certificate becomes valid
	NotBefore time.Time

	// NotAfter is when the certificate expires
	NotAfter time.Time
}

// CertificateManager manages TLS certificates
type CertificateManager struct {
	mu sync.RWMutex

	// certificates maps domain names to certificates
	// Supports exact matches and wildcard patterns
	certificates map[string]*Certificate

	// defaultCert is used when no matching certificate is found
	defaultCert *Certificate
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager() *CertificateManager {
	return &CertificateManager{
		certificates: make(map[string]*Certificate),
	}
}

// LoadCertificate loads a certificate and private key from files
func (cm *CertificateManager) LoadCertificate(certFile, keyFile string) (*Certificate, error) {
	// Load the certificate and private key
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// Parse the certificate to extract information
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Extract domain names from certificate
	domains := make([]string, 0)
	if cert.Subject.CommonName != "" {
		domains = append(domains, cert.Subject.CommonName)
	}
	domains = append(domains, cert.DNSNames...)

	certificate := &Certificate{
		Cert:      cert,
		TLSCert:   tlsCert,
		Domains:   domains,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
	}

	return certificate, nil
}

// AddCertificate adds a certificate for the specified domains
func (cm *CertificateManager) AddCertificate(cert *Certificate) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate certificate
	if err := cm.validateCertificate(cert); err != nil {
		return err
	}

	// Add certificate for each domain
	for _, domain := range cert.Domains {
		cm.certificates[domain] = cert
	}

	// If no default certificate is set, use this one
	if cm.defaultCert == nil {
		cm.defaultCert = cert
	}

	return nil
}

// AddCertificateFromFiles loads and adds a certificate from files
func (cm *CertificateManager) AddCertificateFromFiles(certFile, keyFile string) error {
	cert, err := cm.LoadCertificate(certFile, keyFile)
	if err != nil {
		return err
	}

	return cm.AddCertificate(cert)
}

// SetDefaultCertificate sets the default certificate
func (cm *CertificateManager) SetDefaultCertificate(cert *Certificate) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := cm.validateCertificate(cert); err != nil {
		return err
	}

	cm.defaultCert = cert
	return nil
}

// GetCertificate returns the certificate for the given server name (SNI)
// This method is suitable for use as tls.Config.GetCertificate
func (cm *CertificateManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	serverName := hello.ServerName
	if serverName == "" {
		// No SNI provided, use default certificate
		if cm.defaultCert != nil {
			return &cm.defaultCert.TLSCert, nil
		}
		return nil, fmt.Errorf("no default certificate configured")
	}

	// Try exact match first
	if cert, ok := cm.certificates[serverName]; ok {
		return &cert.TLSCert, nil
	}

	// Try wildcard match
	if cert := cm.findWildcardCertificate(serverName); cert != nil {
		return &cert.TLSCert, nil
	}

	// Fall back to default certificate
	if cm.defaultCert != nil {
		return &cm.defaultCert.TLSCert, nil
	}

	return nil, fmt.Errorf("no certificate found for %s", serverName)
}

// findWildcardCertificate finds a wildcard certificate matching the server name
func (cm *CertificateManager) findWildcardCertificate(serverName string) *Certificate {
	// Try wildcard pattern (e.g., *.example.com)
	// This is a simple implementation - more sophisticated matching could be added
	for domain, cert := range cm.certificates {
		if len(domain) > 2 && domain[0] == '*' && domain[1] == '.' {
			// Wildcard certificate
			suffix := domain[1:] // Remove the '*'
			if len(serverName) > len(suffix) && serverName[len(serverName)-len(suffix):] == suffix {
				// Check that it's a proper subdomain match
				remaining := serverName[:len(serverName)-len(suffix)]
				if remaining[len(remaining)-1] == '.' {
					return cert
				}
			}
		}
	}
	return nil
}

// RemoveCertificate removes a certificate for the specified domain
func (cm *CertificateManager) RemoveCertificate(domain string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.certificates, domain)
}

// ListCertificates returns a list of all managed certificates
func (cm *CertificateManager) ListCertificates() []*Certificate {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	seen := make(map[*Certificate]bool)
	certs := make([]*Certificate, 0)

	for _, cert := range cm.certificates {
		if !seen[cert] {
			certs = append(certs, cert)
			seen[cert] = true
		}
	}

	return certs
}

// CheckExpiry checks for expiring certificates and returns those expiring within the given duration
func (cm *CertificateManager) CheckExpiry(within time.Duration) []*Certificate {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	threshold := time.Now().Add(within)
	expiring := make([]*Certificate, 0)
	seen := make(map[*Certificate]bool)

	for _, cert := range cm.certificates {
		if !seen[cert] && cert.NotAfter.Before(threshold) {
			expiring = append(expiring, cert)
			seen[cert] = true
		}
	}

	return expiring
}

// validateCertificate validates a certificate
func (cm *CertificateManager) validateCertificate(cert *Certificate) error {
	if cert == nil {
		return fmt.Errorf("certificate is nil")
	}

	if cert.Cert == nil {
		return fmt.Errorf("certificate x509 cert is nil")
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (valid from %s)", cert.NotBefore)
	}

	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired (expired on %s)", cert.NotAfter)
	}

	// Check if certificate will expire soon (within 7 days)
	if now.Add(7 * 24 * time.Hour).After(cert.NotAfter) {
		// This is a warning, not an error
		// In production, you should log this
	}

	return nil
}

// GenerateSelfSignedCertificate generates a self-signed certificate for testing
func GenerateSelfSignedCertificate(domains []string) (*Certificate, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Balance Load Balancer"},
			CommonName:   domains[0],
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              domains,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Parse the created certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create tls.Certificate
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	return &Certificate{
		Cert:      cert,
		TLSCert:   tlsCert,
		Domains:   domains,
		NotBefore: notBefore,
		NotAfter:  notAfter,
	}, nil
}

// SaveCertificateToPEM saves a certificate and private key to PEM files
func SaveCertificateToPEM(cert *Certificate, certFile, keyFile string) error {
	// Save certificate
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.TLSCert.Certificate[0],
	}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Save private key
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privateKey, ok := cert.TLSCert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("private key is not RSA")
	}

	if err := pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

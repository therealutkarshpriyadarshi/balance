#!/bin/bash
# Generate self-signed certificates for testing
# DO NOT use these certificates in production!

set -e

# Create certs directory if it doesn't exist
CERTS_DIR="certs"
mkdir -p "$CERTS_DIR"

echo "Generating test certificates for Balance..."
echo "WARNING: These are self-signed certificates for testing only!"
echo ""

# Function to generate a certificate
generate_cert() {
    local domain=$1
    local cert_file="$CERTS_DIR/$domain.crt"
    local key_file="$CERTS_DIR/$domain.key"
    local csr_file="$CERTS_DIR/$domain.csr"

    echo "Generating certificate for $domain..."

    # Generate private key
    openssl genrsa -out "$key_file" 2048

    # Generate certificate signing request
    openssl req -new -key "$key_file" -out "$csr_file" \
        -subj "/C=US/ST=California/L=San Francisco/O=Balance/CN=$domain"

    # Generate self-signed certificate (valid for 1 year)
    openssl x509 -req -days 365 -in "$csr_file" \
        -signkey "$key_file" -out "$cert_file"

    # Clean up CSR
    rm "$csr_file"

    # Set appropriate permissions
    chmod 644 "$cert_file"
    chmod 600 "$key_file"

    echo "  Created: $cert_file"
    echo "  Created: $key_file"
    echo ""
}

# Function to generate a certificate with SANs (Subject Alternative Names)
generate_cert_with_sans() {
    local cn=$1
    shift
    local sans=("$@")
    local cert_file="$CERTS_DIR/$cn.crt"
    local key_file="$CERTS_DIR/$cn.key"
    local csr_file="$CERTS_DIR/$cn.csr"
    local ext_file="$CERTS_DIR/$cn.ext"

    echo "Generating certificate for $cn with SANs..."

    # Generate private key
    openssl genrsa -out "$key_file" 2048

    # Create SAN extension file
    cat > "$ext_file" << EOF
subjectAltName = @alt_names

[alt_names]
EOF

    local i=1
    for san in "${sans[@]}"; do
        echo "DNS.$i = $san" >> "$ext_file"
        ((i++))
    done

    # Generate certificate signing request
    openssl req -new -key "$key_file" -out "$csr_file" \
        -subj "/C=US/ST=California/L=San Francisco/O=Balance/CN=$cn"

    # Generate self-signed certificate with SANs (valid for 1 year)
    openssl x509 -req -days 365 -in "$csr_file" \
        -signkey "$key_file" -out "$cert_file" \
        -extfile "$ext_file"

    # Clean up
    rm "$csr_file" "$ext_file"

    # Set appropriate permissions
    chmod 644 "$cert_file"
    chmod 600 "$key_file"

    echo "  Created: $cert_file (with SANs: ${sans[*]})"
    echo "  Created: $key_file"
    echo ""
}

# Function to generate a wildcard certificate
generate_wildcard_cert() {
    local domain=$1
    local wildcard="*.$domain"
    local cert_file="$CERTS_DIR/wildcard.$domain.crt"
    local key_file="$CERTS_DIR/wildcard.$domain.key"
    local csr_file="$CERTS_DIR/wildcard.$domain.csr"
    local ext_file="$CERTS_DIR/wildcard.$domain.ext"

    echo "Generating wildcard certificate for $wildcard..."

    # Generate private key
    openssl genrsa -out "$key_file" 2048

    # Create SAN extension file
    cat > "$ext_file" << EOF
subjectAltName = @alt_names

[alt_names]
DNS.1 = $wildcard
DNS.2 = $domain
EOF

    # Generate certificate signing request
    openssl req -new -key "$key_file" -out "$csr_file" \
        -subj "/C=US/ST=California/L=San Francisco/O=Balance/CN=$wildcard"

    # Generate self-signed certificate (valid for 1 year)
    openssl x509 -req -days 365 -in "$csr_file" \
        -signkey "$key_file" -out "$cert_file" \
        -extfile "$ext_file"

    # Clean up
    rm "$csr_file" "$ext_file"

    # Set appropriate permissions
    chmod 644 "$cert_file"
    chmod 600 "$key_file"

    echo "  Created: $cert_file"
    echo "  Created: $key_file"
    echo ""
}

# Generate test certificates

# 1. Basic certificate for example.com
generate_cert "localhost"

# 2. Certificate with SANs for example.com
generate_cert_with_sans "example.com" "example.com" "www.example.com"

# 3. API certificate
generate_cert "api.example.com"

# 4. Wildcard certificate
generate_wildcard_cert "example.com"

# 5. Test certificate
generate_cert "test.local"

echo "✅ Test certificates generated successfully!"
echo ""
echo "Certificates location: $CERTS_DIR/"
echo ""
echo "To use these certificates:"
echo "1. Update your config file to point to the certificate files"
echo "2. Add hosts to /etc/hosts (for testing):"
echo "   127.0.0.1 example.com www.example.com api.example.com"
echo "3. Test with curl (use -k to skip certificate verification):"
echo "   curl -k https://localhost:443"
echo ""
echo "⚠️  Remember: These are self-signed certificates for testing only!"
echo "   Do NOT use them in production. Use Let's Encrypt or a proper CA."

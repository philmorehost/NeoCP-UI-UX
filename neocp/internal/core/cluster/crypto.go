package cluster

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// PEMKeyPair holds PEM encoded certificate and private key
type PEMKeyPair struct {
	CertPEM []byte
	KeyPEM  []byte
}

// GenerateClusterKeys creates in-memory CA and signs certificates for both Master and Worker
func GenerateClusterKeys(masterIPs []string, workerIPs []string) (*PEMKeyPair, *PEMKeyPair, *PEMKeyPair, error) {
	// 1. Generate CA key pair
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2026052001),
		Subject: pkix.Name{
			Organization: []string{"NeoCP Professional CA"},
			CommonName:   "NeoCP Root CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})
	caPrivBytes, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caPrivBytes})
	caKeyPair := &PEMKeyPair{CertPEM: caCertPEM, KeyPEM: caKeyPEM}

	// 2. Generate Master (Server) key pair
	masterKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate Master key: %w", err)
	}

	masterTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2026052002),
		Subject: pkix.Name{
			Organization: []string{"NeoCP Master Server"},
			CommonName:   "neocp-master",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	// Set IP addresses for SAN validation
	for _, ipStr := range masterIPs {
		if ip := net.ParseIP(ipStr); ip != nil {
			masterTemplate.IPAddresses = append(masterTemplate.IPAddresses, ip)
		}
	}
	masterTemplate.IPAddresses = append(masterTemplate.IPAddresses, net.ParseIP("127.0.0.1"))

	masterCertBytes, err := x509.CreateCertificate(rand.Reader, masterTemplate, caTemplate, &masterKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to sign Master certificate: %w", err)
	}

	masterCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: masterCertBytes})
	masterPrivBytes, err := x509.MarshalECPrivateKey(masterKey)
	if err != nil {
		return nil, nil, nil, err
	}
	masterKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: masterPrivBytes})
	masterKeyPair := &PEMKeyPair{CertPEM: masterCertPEM, KeyPEM: masterKeyPEM}

	// 3. Generate Worker (Client) key pair
	workerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate Worker key: %w", err)
	}

	workerTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2026052003),
		Subject: pkix.Name{
			Organization: []string{"NeoCP Worker Node"},
			CommonName:   "neocp-worker",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	for _, ipStr := range workerIPs {
		if ip := net.ParseIP(ipStr); ip != nil {
			workerTemplate.IPAddresses = append(workerTemplate.IPAddresses, ip)
		}
	}
	workerTemplate.IPAddresses = append(workerTemplate.IPAddresses, net.ParseIP("127.0.0.1"))

	workerCertBytes, err := x509.CreateCertificate(rand.Reader, workerTemplate, caTemplate, &workerKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to sign Worker certificate: %w", err)
	}

	workerCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: workerCertBytes})
	workerPrivBytes, err := x509.MarshalECPrivateKey(workerKey)
	if err != nil {
		return nil, nil, nil, err
	}
	workerKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: workerPrivBytes})
	workerKeyPair := &PEMKeyPair{CertPEM: workerCertPEM, KeyPEM: workerKeyPEM}

	return caKeyPair, masterKeyPair, workerKeyPair, nil
}

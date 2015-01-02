package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
)

type Client struct {
	serverAddr         string
	sslCertificatePath string
	sslKeyPath         string
}

func NewClient(serverAddr string, sslCertificatePath string, sslKeyPath string) (*Client, error) {
	if sslCertificatePath == "" || sslKeyPath == "" {
		return nil, fmt.Errorf("ssl certificate and key path must be specified")
	}

	c := &Client{
		serverAddr:         serverAddr,
		sslCertificatePath: sslCertificatePath,
		sslKeyPath:         sslKeyPath,
	}

	return c, nil
}

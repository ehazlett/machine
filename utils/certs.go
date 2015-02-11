package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func newCertificate(org string) (*x509.Certificate, error) {
	now := time.Now()
	// need to set notBefore slightly in the past to account for time
	// skew in the VMs otherwise the certs sometimes are not yet valid
	notBefore := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-5, 0, 0, time.Local)
	notAfter := notBefore.Add(time.Hour * 24 * 1080)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}, nil

}

// GenerateCACertificate generates a new certificate authority from the specified org
// and bit size and stores the resulting certificate and key file
// in the arguments.
func GenerateCACertificate(certFile, keyFile, org string, bits int) error {
	template, err := newCertificate(org)
	if err != nil {
		return err
	}

	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign

	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err

	}

	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	return nil
}

// GenerateCert generates a new certificate signed using the provided
// certificate authority files and stores the result in the certificate
// file and key provided.  The provided host names are set to the
// appropriate certificate fields.
func GenerateCert(hosts []string, certFile, keyFile, caFile, caKeyFile, org string, bits int) error {
	template, err := newCertificate(org)
	if err != nil {
		return err
	}
	// client
	if len(hosts) == 1 && hosts[0] == "" {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
		template.KeyUsage = x509.KeyUsageDigitalSignature
	} else { // server
		for _, h := range hosts {
			if ip := net.ParseIP(h); ip != nil {
				template.IPAddresses = append(template.IPAddresses, ip)

			} else {
				template.DNSNames = append(template.DNSNames, h)
			}
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(caFile, caKeyFile)
	if err != nil {
		return err

	}

	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err

	}

	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, x509Cert, &priv.PublicKey, tlsCert.PrivateKey)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err

	}

	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err

	}

	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	return nil
}

func SetupMachineCertificates(caCertPath, caKeyPath, clientCertPath, clientKeyPath string) error {
	org := GetUsername()
	bits := 2048

	if _, err := os.Stat(GetMachineDir()); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(GetMachineDir(), 0700); err != nil {
				return fmt.Errorf("Error creating machine config dir: %s", err)
			}
		} else {
			return err
		}
	}

	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		// check if the key path exists; if so, error
		if _, err := os.Stat(caKeyPath); err == nil {
			return fmt.Errorf("The CA key already exists.  Please remove it or specify a different key/cert.")
		}

		if err := GenerateCACertificate(caCertPath, caKeyPath, org, bits); err != nil {
			return fmt.Errorf("Error generating CA certificate: %s", err)
		}
	}

	if _, err := os.Stat(clientCertPath); os.IsNotExist(err) {
		if _, err := os.Stat(GetMachineClientCertDir()); err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(GetMachineClientCertDir(), 0700); err != nil {
					return fmt.Errorf("Error creating machine client cert dir: %s", err)
				}
			} else {
				return err
			}
		}

		// check if the key path exists; if so, error
		if _, err := os.Stat(clientKeyPath); err == nil {
			return fmt.Errorf("The client key already exists.  Please remove it or specify a different key/cert.")
		}

		if err := GenerateCert([]string{""}, clientCertPath, clientKeyPath, caCertPath, caKeyPath, org, bits); err != nil {
			return fmt.Errorf("Error generating client certificate: %s", err)
		}

		// copy ca.pem to client cert dir for docker client
		if err := CopyFile(caCertPath, filepath.Join(GetMachineClientCertDir(), "ca.pem")); err != nil {
			return fmt.Errorf("Error copying ca.pem to client cert dir: %s", err)
		}
	}

	return nil
}

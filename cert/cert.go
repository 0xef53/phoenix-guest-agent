package cert

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

type Store interface {
	ReadFile(string) ([]byte, error)
}

type Dir string

func (d Dir) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(d), name))
}

func NewClientConfig(store Store) (*tls.Config, error) {
	return newConfig(store, "client")
}

func NewServerConfig(store Store) (*tls.Config, error) {
	cfg, err := newConfig(store, "agent")
	if err != nil {
		return nil, err
	}

	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	cfg.ClientCAs = cfg.RootCAs

	return cfg, nil
}

func newConfig(store Store, name string) (*tls.Config, error) {
	if store == nil {
		return nil, fmt.Errorf("no certificate store defined")
	}

	certPool := x509.NewCertPool()

	ca, err := store.ReadFile("CA.crt")
	if err != nil {
		return nil, fmt.Errorf("unable to read CA certificate: %s", err)
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, fmt.Errorf("failed to append client certificates from CA.crt")
	}

	crt, err := store.ReadFile(name + ".crt")
	if err != nil {
		return nil, fmt.Errorf("unable to load crt file: %s", err)
	}

	key, err := store.ReadFile(name + ".key")
	if err != nil {
		return nil, fmt.Errorf("unable to load key file: %s", err)
	}

	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:             []tls.Certificate{cert},
		RootCAs:                  certPool,
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		ClientSessionCache:       tls.NewLRUClientSessionCache(0),
		NextProtos:               []string{"h2"},
	}, nil
}

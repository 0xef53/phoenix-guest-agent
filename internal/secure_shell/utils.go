package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

func generateSigner() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(key)
}

func parsePtyRequest(s []byte) (pty Pty, ok bool) {
	term, s, ok := parseString(s)
	if !ok {
		return
	}

	width32, s, ok := parseUint32(s)
	if !ok {
		return
	}

	height32, _, ok := parseUint32(s)
	if !ok {
		return
	}

	pty = Pty{
		Term: term,
		Window: Window{
			Width:  int(width32),
			Height: int(height32),
		},
	}

	return
}

func parseWinchRequest(s []byte) (win Window, ok bool) {
	width32, s, ok := parseUint32(s)
	if width32 < 1 {
		ok = false
	}
	if !ok {
		return
	}

	height32, _, ok := parseUint32(s)
	if height32 < 1 {
		ok = false
	}
	if !ok {
		return
	}

	win = Window{
		Width:  int(width32),
		Height: int(height32),
	}

	return
}

func parseString(in []byte) (out string, rest []byte, ok bool) {
	if len(in) < 4 {
		return
	}

	length := binary.BigEndian.Uint32(in)
	if uint32(len(in)) < 4+length {
		return
	}

	out = string(in[4 : 4+length])

	rest = in[4+length:]

	ok = true

	return
}

func parseUint32(in []byte) (uint32, []byte, bool) {
	if len(in) < 4 {
		return 0, nil, false
	}

	return binary.BigEndian.Uint32(in), in[4:], true
}

// GeneratePrivateKey creates an RSA Private Key of specified byte size
func GeneratePrivateKey(bytesize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bytesize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// EncodePrivateKeyToPEM encodes Private Key from RSA to PEM format
func EncodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	return pem.EncodeToMemory(&privBlock)
}

// GeneratePublicKey takes an rsa.PublicKey and returns bytes
// suitable for writing to .pub file in the format "ssh-rsa ..."
func GeneratePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	return ssh.MarshalAuthorizedKey(publicRsaKey), nil
}

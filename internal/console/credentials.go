package console

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
)

// Both password and key credentials live inside the existing encrypted vault.
// Legacy raw private-key records remain readable without a database rewrite.
type sshCredential struct {
	Version    int    `json:"version"`
	Method     string `json:"method"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

func credentialFromHost(h Host) ([]byte, error) {
	method := h.AuthMethod
	if method == "" {
		method = "key"
		if h.Password != "" {
			method = "password"
		}
	}
	c := sshCredential{Version: 1, Method: method}
	switch method {
	case "password":
		if h.Password == "" || len(h.Password) > 4096 {
			return nil, fmt.Errorf("پسورد SSH لازم است")
		}
		c.Password = h.Password
	case "key":
		c.PrivateKey = h.PrivateKey
		c.Passphrase = h.KeyPassphrase
	default:
		return nil, fmt.Errorf("روش ورود SSH معتبر نیست")
	}
	b, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	_, e = credentialAuth(b)
	return b, e
}
func readCredential(b []byte) (sshCredential, error) {
	var c sshCredential
	if !bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		return sshCredential{Version: 1, Method: "key", PrivateKey: string(b)}, nil
	}
	if e := json.Unmarshal(b, &c); e != nil || c.Version != 1 {
		return c, fmt.Errorf("invalid stored SSH credential")
	}
	return c, nil
}
func credentialAuth(b []byte) (ssh.AuthMethod, error) {
	c, e := readCredential(b)
	if e != nil {
		return nil, e
	}
	if c.Method == "password" && c.Password != "" {
		return ssh.Password(c.Password), nil
	}
	if c.Method != "key" {
		return nil, fmt.Errorf("invalid SSH authentication method")
	}
	var signer ssh.Signer
	if c.Passphrase != "" {
		signer, e = ssh.ParsePrivateKeyWithPassphrase([]byte(c.PrivateKey), []byte(c.Passphrase))
	} else {
		signer, e = ssh.ParsePrivateKey([]byte(c.PrivateKey))
	}
	if e != nil {
		return nil, fmt.Errorf("کلید خصوصی SSH یا رمز کلید معتبر نیست")
	}
	return ssh.PublicKeys(signer), nil
}

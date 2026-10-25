package console

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"testing"
)

func TestEncryptedPrivateKeyAndInvalidCredentials(t *testing.T) {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	block, e := ssh.MarshalPrivateKeyWithPassphrase(key, "local-test", []byte("key-test-password"))
	if e != nil {
		t.Fatal(e)
	}
	h := Host{AuthMethod: "key", PrivateKey: string(pem.EncodeToMemory(block)), KeyPassphrase: "key-test-password"}
	encoded, e := credentialFromHost(h)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = credentialAuth(encoded); e != nil {
		t.Fatal(e)
	}
	h.KeyPassphrase = "incorrect"
	if _, e = credentialFromHost(h); e == nil {
		t.Fatal("incorrect key passphrase accepted")
	}
	h = Host{AuthMethod: "password"}
	if _, e = credentialFromHost(h); e == nil {
		t.Fatal("empty password accepted")
	}
	h = Host{AuthMethod: "unsupported"}
	if _, e = credentialFromHost(h); e == nil {
		t.Fatal("unsupported method accepted")
	}
	if _, e = credentialAuth([]byte(`{"version":2,"method":"password","password":"test"}`)); e == nil {
		t.Fatal("future envelope accepted")
	}
}

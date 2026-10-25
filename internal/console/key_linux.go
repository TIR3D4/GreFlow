package console

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

func loadKey(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
		b = make([]byte, 32)
		if _, e = rand.Read(b); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.Write(b)
		ce := f.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
	} else if e != nil {
		return nil, e
	}
	st, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if st.Mode().Perm()&0077 != 0 || len(b) != 32 {
		return nil, fmt.Errorf("key must contain 32 bytes with mode 0600")
	}
	return b, nil
}

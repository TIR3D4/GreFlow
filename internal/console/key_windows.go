package console

import (
	"crypto/rand"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"unsafe"
)

func loadKey(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		plain := make([]byte, 32)
		if _, e = rand.Read(plain); e != nil {
			return nil, e
		}
		in := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
		var out windows.DataBlob
		if e = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
			return nil, e
		}
		defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
		b = append([]byte{}, unsafe.Slice(out.Data, out.Size)...)
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
		if e = os.WriteFile(path, b, 0600); e != nil {
			return nil, e
		}
	} else if e != nil {
		return nil, e
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("empty wrapped key")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
		return nil, e
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	key := append([]byte{}, unsafe.Slice(out.Data, out.Size)...)
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid master key")
	}
	return key, nil
}

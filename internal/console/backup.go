package console

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/scrypt"
	"net/http"
	"time"
)

type backupHost struct {
	Host Host   `json:"host"`
	Key  []byte `json:"key,omitempty"`
}
type backupData struct {
	Snapshot  map[string]snapshotTable `json:"snapshot,omitempty"`
	ExportKey []byte                   `json:"export_key,omitempty"`
	Version   int                      `json:"version"`
	At        time.Time                `json:"at"`
	Secrets   bool                     `json:"secrets"`
	Hosts     []backupHost             `json:"hosts"`
}

func backupCipher(password string, salt []byte) (cipher.AEAD, error) {
	key, e := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if e != nil {
		return nil, e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func sealBackup(v backupData, password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	a, e := backupCipher(password, salt)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, a.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	header := append([]byte("GFB1"), salt...)
	header = append(header, nonce...)
	data := a.Seal(header, nonce, []byte(marshal(v)), []byte("GFB1"))
	return base64.StdEncoding.EncodeToString(data), nil
}
func openBackup(data, password string) (backupData, error) {
	var out backupData
	b, e := base64.StdEncoding.DecodeString(data)
	if e != nil || len(b) < 48 || string(b[:4]) != "GFB1" {
		return out, fmt.Errorf("invalid backup format")
	}
	a, e := backupCipher(password, b[4:20])
	if e != nil {
		return out, e
	}
	plain, e := a.Open(nil, b[20:32], b[32:], []byte("GFB1"))
	if e != nil {
		return out, fmt.Errorf("backup password or integrity check failed")
	}
	e = json.Unmarshal(plain, &out)
	if e == nil && out.Version != 1 {
		e = fmt.Errorf("unsupported backup version")
	}
	return out, e
}
func (a *App) backup(w http.ResponseWriter, r *http.Request, u User, path string) {
	var v struct {
		Password       string `json:"password"`
		IncludeSecrets bool   `json:"include_secrets"`
		Scope          string `json:"scope"`
		Data           string `json:"data"`
		Revision       string `json:"revision"`
	}
	if !decode(w, r, &v) {
		return
	}
	if len(v.Password) < 16 {
		apiError(w, 400, "PASSWORD", "رمز بکاپ باید حداقل ۱۶ نویسه باشد")
		return
	}
	if path == "backups" && v.Scope == "full" {
		a.fullBackup(w, r, u, v.Password)
		return
	}
	if path == "backups" {
		// The single read transaction is a consistent logical inventory snapshot.
		tx, e := a.Store.DB.Begin()
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		defer tx.Rollback()
		rows, e := tx.Query("SELECT id,name,kind,addresses,hostname,port,username,fingerprint,credential FROM hosts")
		if e != nil {
			apiError(w, 500, "STORAGE", e.Error())
			return
		}
		b := backupData{Version: 1, At: time.Now().UTC(), Secrets: v.IncludeSecrets, Hosts: []backupHost{}}
		for rows.Next() {
			var h backupHost
			var addresses string
			var key []byte
			if e = rows.Scan(&h.Host.ID, &h.Host.Name, &h.Host.Kind, &addresses, &h.Host.Hostname, &h.Host.Port, &h.Host.Username, &h.Host.Fingerprint, &key); e != nil {
				rows.Close()
				apiError(w, 500, "STORAGE", e.Error())
				return
			}
			json.Unmarshal([]byte(addresses), &h.Host.Addresses)
			if v.IncludeSecrets {
				h.Key, e = a.Store.Open(key)
				if e != nil {
					rows.Close()
					apiError(w, 500, "KEY", e.Error())
					return
				}
			}
			b.Hosts = append(b.Hosts, h)
		}
		rows.Close()
		tx.Rollback()
		data, e := sealBackup(b, v.Password)
		if e != nil {
			apiError(w, 500, "BACKUP", e.Error())
			return
		}
		a.Store.Audit(u.ID, "backup", "")
		reply(w, 200, map[string]string{"data": data, "scope": "host inventory and optional SSH credentials; remote configs stay on hosts; accounts, sessions and history are excluded"})
		return
	}
	b, e := openBackup(v.Data, v.Password)
	if e != nil {
		apiError(w, 400, "BACKUP", e.Error())
		return
	}
	if b.Snapshot != nil {
		var count int
		a.Store.DB.QueryRow("SELECT count(*) FROM hosts").Scan(&count)
		rev := hash(v.Data)
		if path == "restore-plans" {
			reply(w, 200, map[string]any{"revision": rev, "hosts": len(b.Snapshot["hosts"].Rows), "includes_secrets": true, "can_restore": count == 0, "scope": "full", "effect": "replaces accounts, inventory, CA and history; logs out every session; old queued/running operations become uncertain; no remote changes"})
			return
		}
		if v.Revision != rev || count != 0 {
			apiError(w, 409, "RESTORE_CONFLICT", "پنل بدون سرور و تأیید plan لازم است")
			return
		}
		if e = a.Store.restoreSnapshot(b); e != nil {
			apiError(w, 409, "RESTORE", e.Error())
			return
		}
		a.Store.Audit(u.ID, "restore_full", rev)
		reply(w, 200, map[string]any{"restored": len(b.Snapshot["hosts"].Rows), "remote_changes": false, "logout_required": true})
		return
	}
	for _, h := range b.Hosts {
		if e = h.Host.Validate(); e != nil {
			apiError(w, 400, "BACKUP", e.Error())
			return
		}
	}
	var count int
	a.Store.DB.QueryRow("SELECT count(*) FROM hosts").Scan(&count)
	rev := hash(v.Data)
	if path == "restore-plans" {
		reply(w, 200, map[string]any{"revision": rev, "hosts": len(b.Hosts), "includes_secrets": b.Secrets, "can_restore": count == 0 && b.Secrets, "effect": "imports management inventory only; remote hosts and tunnels are not modified; existing accounts remain"})
		return
	}
	if v.Revision != rev || count != 0 || !b.Secrets {
		apiError(w, 409, "RESTORE_CONFLICT", "بازیابی نیازمند پنل بدون سرور، بکاپ دارای کلید و تأیید پیش‌نمایش است")
		return
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	defer tx.Rollback()
	for _, bh := range b.Hosts {
		h := bh.Host
		key, e := a.Store.Seal(bh.Key)
		if e != nil {
			apiError(w, 500, "KEY", e.Error())
			return
		}
		_, e = tx.Exec("INSERT INTO hosts(id,name,kind,addresses,hostname,port,username,fingerprint,credential) VALUES(?,?,?,?,?,?,?,?,?)", ID(), h.Name, h.Kind, marshal(h.Addresses), h.Hostname, h.Port, h.Username, h.Fingerprint, key)
		if e != nil {
			apiError(w, 409, "RESTORE", e.Error())
			return
		}
	}
	if e = tx.Commit(); e != nil {
		apiError(w, 500, "STORAGE", e.Error())
		return
	}
	a.Store.Audit(u.ID, "restore_inventory", rev)
	reply(w, 200, map[string]any{"restored": len(b.Hosts), "remote_changes": false})
}

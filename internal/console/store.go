package console

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	DB  *sql.DB
	Key []byte
	Dir string
}

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func OpenStore(dir, keyPath string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	key, e := loadKey(keyPath)
	if e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "console.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, Dir: dir, Key: key}
	if e = s.migrate(); e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) migrate() error {
	if _, e := s.DB.Exec("CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY)"); e != nil {
		return e
	}
	var existing sql.NullInt64
	if e := s.DB.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&existing); e != nil {
		return e
	}
	if existing.Valid && existing.Int64 > 4 {
		return fmt.Errorf("database schema %d is newer than this binary; restore the matching snapshot for downgrade", existing.Int64)
	}
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, e := s.DB.Exec(q); e != nil {
			return e
		}
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY);
 CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, password BLOB NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','operator','viewer')), scope TEXT NOT NULL DEFAULT '[]', totp BLOB, last_totp INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS recovery(user_id TEXT REFERENCES users(id), code TEXT UNIQUE NOT NULL);
 CREATE TABLE IF NOT EXISTS mutation_receipts(actor TEXT NOT NULL,idem TEXT NOT NULL,digest TEXT NOT NULL,response BLOB,created_at TEXT NOT NULL,PRIMARY KEY(actor,idem));
 CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES users(id),csrf TEXT NOT NULL,expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS hosts(id TEXT PRIMARY KEY,name TEXT NOT NULL,kind TEXT NOT NULL,addresses TEXT NOT NULL,hostname TEXT NOT NULL,port INTEGER NOT NULL,username TEXT NOT NULL,fingerprint TEXT NOT NULL,credential BLOB NOT NULL,host_identity TEXT UNIQUE,inventory TEXT,last_error TEXT,observed_at TEXT);
 CREATE TABLE IF NOT EXISTS operations(id TEXT PRIMARY KEY,actor TEXT NOT NULL,host_id TEXT NOT NULL REFERENCES hosts(id),idem TEXT NOT NULL,request TEXT NOT NULL,state TEXT NOT NULL,result TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(actor,idem));
 CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT,operation_id TEXT NOT NULL REFERENCES operations(id),at TEXT NOT NULL,state TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY AUTOINCREMENT,actor TEXT NOT NULL,action TEXT NOT NULL,object_id TEXT NOT NULL,at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS schedules(id TEXT PRIMARY KEY,host_id TEXT NOT NULL REFERENCES hosts(id),body TEXT NOT NULL,operation_id TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(id TEXT PRIMARY KEY,body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS agents(host_id TEXT PRIMARY KEY REFERENCES hosts(id), serial TEXT UNIQUE NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, heartbeat TEXT);
 CREATE TABLE IF NOT EXISTS agent_certificates(serial TEXT PRIMARY KEY,host_id TEXT NOT NULL REFERENCES hosts(id),expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS enrollments(id TEXT PRIMARY KEY, host_id TEXT NOT NULL REFERENCES hosts(id), token_hash TEXT NOT NULL UNIQUE, expires INTEGER NOT NULL, used INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS incidents(id TEXT PRIMARY KEY,state TEXT NOT NULL,generation INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS deliveries(id TEXT PRIMARY KEY,incident TEXT NOT NULL UNIQUE,body TEXT NOT NULL,state TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,next_at INTEGER NOT NULL);
 INSERT OR IGNORE INTO schema_migrations(version) VALUES(1);
 INSERT OR IGNORE INTO schema_migrations(version) VALUES(2);
 INSERT OR IGNORE INTO schema_migrations(version) VALUES(3);`)
	if e != nil {
		return e
	}
	// User-requested retirement of TOTP; leave old columns for backup compatibility.
	if !existing.Valid || existing.Int64 < 4 {
		if _, e = tx.Exec("UPDATE users SET totp=NULL,last_totp=0; DELETE FROM recovery; INSERT OR IGNORE INTO schema_migrations(version) VALUES(4)"); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) Seal(b []byte) ([]byte, error) {
	block, e := aes.NewCipher(s.Key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	n := make([]byte, a.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return nil, e
	}
	return a.Seal(n, n, b, []byte("greflow-console-v1")), nil
}
func (s *Store) Open(b []byte) ([]byte, error) {
	block, e := aes.NewCipher(s.Key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	if len(b) < a.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted value")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte("greflow-console-v1"))
}
func (s *Store) Audit(actor, action, id string) error {
	_, e := s.DB.Exec("INSERT INTO audit(actor,action,object_id,at) VALUES(?,?,?,?)", actor, action, id, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}
func marshal(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return string(b)
}

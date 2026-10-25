package console

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type snapshotCell struct {
	Kind   string `json:"kind"`
	Text   string `json:"text,omitempty"`
	Number int64  `json:"number,omitempty"`
}
type snapshotTable struct {
	Columns []string         `json:"columns"`
	Rows    [][]snapshotCell `json:"rows"`
}

var snapshotTables = []string{"users", "recovery", "hosts", "operations", "events", "audit", "schedules", "settings", "agents", "agent_certificates", "enrollments", "incidents", "deliveries"}

func (s *Store) snapshot() (map[string]snapshotTable, error) {
	tx, e := s.DB.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	out := map[string]snapshotTable{}
	for _, table := range snapshotTables {
		rows, e := tx.Query("SELECT * FROM " + table)
		if e != nil {
			return nil, e
		}
		cols, e := rows.Columns()
		if e != nil {
			rows.Close()
			return nil, e
		}
		data := snapshotTable{Columns: cols, Rows: [][]snapshotCell{}}
		for rows.Next() {
			values := make([]any, len(cols))
			pointers := make([]any, len(cols))
			for i := range values {
				pointers[i] = &values[i]
			}
			if e = rows.Scan(pointers...); e != nil {
				rows.Close()
				return nil, e
			}
			cells := make([]snapshotCell, len(cols))
			for i, v := range values {
				switch x := v.(type) {
				case nil:
					cells[i].Kind = "null"
				case []byte:
					cells[i] = snapshotCell{Kind: "blob", Text: base64.StdEncoding.EncodeToString(x)}
				case string:
					cells[i] = snapshotCell{Kind: "text", Text: x}
				case int64:
					cells[i] = snapshotCell{Kind: "integer", Number: x}
				default:
					rows.Close()
					return nil, fmt.Errorf("unsupported storage type in %s", table)
				}
			}
			data.Rows = append(data.Rows, cells)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		out[table] = data
	}
	return out, nil
}
func (s *Store) restoreSnapshot(b backupData) error {
	if len(b.ExportKey) != 32 || len(b.Snapshot) != len(snapshotTables) {
		return fmt.Errorf("incomplete full snapshot")
	}
	old := &Store{Key: b.ExportKey}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRow("SELECT count(*) FROM hosts").Scan(&count); e != nil || count != 0 {
		return fmt.Errorf("full restore requires empty host inventory")
	}
	if _, e = tx.Exec("DELETE FROM sessions"); e != nil {
		return e
	}
	for i := len(snapshotTables) - 1; i >= 0; i-- {
		if _, e = tx.Exec("DELETE FROM " + snapshotTables[i]); e != nil {
			return e
		}
	}
	for _, name := range snapshotTables {
		table, ok := b.Snapshot[name]
		if !ok {
			return fmt.Errorf("missing table %s", name)
		}
		rows, e := tx.Query("SELECT * FROM " + name + " LIMIT 0")
		if e != nil {
			return e
		}
		cols, e := rows.Columns()
		rows.Close()
		if e != nil {
			return e
		}
		if strings.Join(cols, ",") != strings.Join(table.Columns, ",") {
			return fmt.Errorf("schema mismatch: %s", name)
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
		query := "INSERT INTO " + name + "(" + strings.Join(cols, ",") + ") VALUES(" + marks + ")"
		for _, row := range table.Rows {
			if len(row) != len(cols) {
				return fmt.Errorf("malformed row")
			}
			values := make([]any, len(cols))
			for i, c := range row {
				switch c.Kind {
				case "null":
					values[i] = nil
				case "text":
					values[i] = c.Text
				case "integer":
					values[i] = c.Number
				case "blob":
					values[i], e = base64.StdEncoding.DecodeString(c.Text)
					if e != nil {
						return e
					}
				default:
					return fmt.Errorf("invalid storage type")
				}
			}
			for i, column := range cols {
				encrypted := name == "hosts" && column == "credential" || name == "users" && column == "totp"
				if name == "settings" && column == "body" {
					encrypted = values[0] == "telegram" || values[0] == "agent-ca"
				}
				if encrypted && values[i] != nil {
					blob, ok := values[i].([]byte)
					if !ok {
						return fmt.Errorf("invalid vault blob")
					}
					plain, e := old.Open(blob)
					if e != nil {
						return e
					}
					values[i], e = s.Seal(plain)
					if e != nil {
						return e
					}
				}
			}
			if _, e = tx.Exec(query, values...); e != nil {
				return fmt.Errorf("restore %s: %w", name, e)
			}
		}
	}
	// Restoration never resumes an old mutation automatically or reuses sessions/tokens.
	if _, e = tx.Exec("UPDATE operations SET state='uncertain',updated_at=? WHERE state IN ('queued','running')", time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE enrollments SET used=1"); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE deliveries SET state='muted' WHERE state='pending'"); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE users SET totp=NULL,last_totp=0; DELETE FROM recovery"); e != nil {
		return e
	}
	return tx.Commit()
}
func (a *App) fullBackup(w http.ResponseWriter, r *http.Request, u User, password string) {
	tables, e := a.Store.snapshot()
	if e != nil {
		apiError(w, 500, "SNAPSHOT", e.Error())
		return
	}
	b := backupData{Version: 1, At: time.Now().UTC(), Secrets: true, Snapshot: tables, ExportKey: a.Store.Key}
	data, e := sealBackup(b, password)
	if e != nil {
		apiError(w, 500, "BACKUP", e.Error())
		return
	}
	if len(data) > 7<<20 {
		apiError(w, 413, "BACKUP_SIZE", "برای این حجم از snapshot خاموش دیتابیس استفاده کنید")
		return
	}
	a.Store.Audit(u.ID, "full_backup", "")
	reply(w, 200, map[string]string{"data": data, "scope": "full control-plane snapshot; sessions excluded; no remote kernel state"})
}

// Package testpg is used exclusively by integration tests, never by the binary.
package testpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/endview/affectweave/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func Open(t *testing.T) (*storage.Store, string) {
	t.Helper()
	path := os.Getenv("FREEAGENT_PG_SERVICE_FILE")
	if path == "" {
		t.Skip("NOT_RUN: isolated PG service unavailable; run through FreeAgent scripts/test-postgres.py")
	}
	var c storage.Connection
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &c) != nil {
		t.Fatal("invalid test service")
	}
	var random [12]byte
	_, _ = rand.Read(random[:])
	suffix := hex.EncodeToString(random[:])
	name := "awtest_" + suffix
	password := suffix + suffix
	config, _ := pgx.ParseConfig("host=127.0.0.1 sslmode=disable")
	config.Host, config.Port, config.User, config.Password, config.Database = c.Host, uint16(c.Port), c.User, c.Password, c.Database
	admin := stdlib.OpenDB(*config)
	t.Cleanup(func() { admin.Close() })
	ctx := t.Context()
	if _, err = admin.ExecContext(ctx, `CREATE ROLE `+name+` LOGIN PASSWORD '`+password+`' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+name+` TEMPLATE template0 ENCODING 'UTF8'`); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.ExecContext(ctx, `REVOKE ALL ON DATABASE `+name+` FROM PUBLIC; GRANT CONNECT ON DATABASE `+name+` TO `+name); err != nil {
		t.Fatal(err)
	}
	config.Database = name
	db := stdlib.OpenDB(*config)
	if _, err = db.ExecContext(ctx, `REVOKE CREATE ON SCHEMA public FROM PUBLIC; CREATE SCHEMA aw AUTHORIZATION `+name); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	c.User, c.Password, c.Database, c.CoreID = name, password, name, "core-test"
	private := filepath.Join(t.TempDir(), "postgres.json")
	b, _ = json.Marshal(c)
	if err = os.WriteFile(private, b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := storage.Open(ctx, private)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE `+name+` WITH (FORCE)`)
		_, _ = admin.ExecContext(context.Background(), `DROP ROLE `+name)
	})
	return s, private
}

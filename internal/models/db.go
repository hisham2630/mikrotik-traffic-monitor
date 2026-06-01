package models

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"mikrotik-monitor/internal/crypto"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
	secret string
}

func Open(path, cliSecret string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory %q: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	d := &DB{DB: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	secret, err := d.resolveSecret(cliSecret)
	if err != nil {
		db.Close()
		return nil, err
	}
	d.secret = secret
	return d, nil
}

// resolveSecret uses the key persisted in app_settings so restarts without -secret
// do not generate a new random key and break decryption of stored device passwords.
func (d *DB) resolveSecret(cliSecret string) (string, error) {
	var stored sql.NullString
	rowErr := d.QueryRow(`SELECT server_secret FROM app_settings WHERE id = 1`).Scan(&stored)
	if rowErr != nil && !errors.Is(rowErr, sql.ErrNoRows) {
		return "", rowErr
	}

	if rowErr == nil && stored.Valid && stored.String != "" {
		if cliSecret != "" && cliSecret != stored.String {
			return "", fmt.Errorf("configured server secret does not match this database; omit -secret to use the key stored in the database")
		}
		return stored.String, nil
	}

	s, err := crypto.EnsureSecret(cliSecret)
	if err != nil {
		return "", err
	}
	if errors.Is(rowErr, sql.ErrNoRows) {
		return s, nil
	}
	_, _ = d.Exec(`UPDATE app_settings SET server_secret = ? WHERE id = 1`, s)
	return s, nil
}

func (d *DB) Secret() string { return d.secret }

func (d *DB) migrate() error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if err := d.bootstrapMigrations(names); err != nil {
		return err
	}
	for _, name := range names {
		applied, err := d.migrationApplied(name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(data)) {
			if _, err := d.Exec(stmt); err != nil {
				return fmt.Errorf("migration %s: %w\nstmt: %s", name, err, stmt)
			}
		}
		if _, err := d.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			return err
		}
	}
	if err := d.PruneExpiredSessions(); err != nil {
		return err
	}
	return d.seed()
}

func (d *DB) migrationApplied(name string) (bool, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&n)
	return n > 0, err
}

// bootstrapMigrations marks pre-tracking migrations as applied on existing databases.
func (d *DB) bootstrapMigrations(names []string) error {
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var devTable int
	if err := d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='devices'`).Scan(&devTable); err != nil {
		return err
	}
	if devTable == 0 {
		return nil
	}
	for _, name := range names {
		if name == "003_notification_channels.sql" {
			continue
		}
		if _, err := d.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			return err
		}
	}
	return nil
}

func splitSQL(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (d *DB) seed() error {
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		hash, err := hashPassword("admin")
		if err != nil {
			return err
		}
		_, err = d.Exec(`INSERT INTO users (username, password_hash, role, must_change_password) VALUES (?, ?, 'admin', 1)`,
			"admin", hash)
		if err != nil {
			return err
		}
	}
	var nc int
	if err := d.QueryRow(`SELECT COUNT(*) FROM notification_config`).Scan(&nc); err != nil {
		return err
	}
	if nc == 0 {
		_, err := d.Exec(`INSERT INTO notification_config (id, api_url_template, phone_numbers, message_template, enabled, whatsapp_enabled, telegram_bot_token, telegram_chat_ids, telegram_enabled) VALUES (1, '', '', '{message}', 0, 0, '', '', 0)`)
		if err != nil {
			return err
		}
	}
	var sc int
	if err := d.QueryRow(`SELECT COUNT(*) FROM app_settings`).Scan(&sc); err != nil {
		return err
	}
	if sc == 0 {
		_, err := d.Exec(`INSERT INTO app_settings (id, retention_days, server_secret) VALUES (1, 7, ?)`, d.secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	if t.IsZero() {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

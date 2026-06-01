package models

import (
	"path/filepath"
	"testing"
)

func TestRepairSkippedMigrationsNoOpOnCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.db")
	db, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.repairSkippedMigrations(); err != nil {
		t.Fatal(err)
	}
	hasWA, err := db.tableHasColumn("alert_rules", "notify_whatsapp")
	if err != nil || !hasWA {
		t.Fatalf("notify_whatsapp: has=%v err=%v", hasWA, err)
	}
}

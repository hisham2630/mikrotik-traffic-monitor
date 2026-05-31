package config

import (
	"flag"
	"os"
)

type Config struct {
	ListenAddr string
	DBPath     string
	SecretKey  string
}

func Load() *Config {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "data.db", "SQLite database path")
	secret := flag.String("secret", "", "Server secret for encryption/JWT (auto-generated if empty)")
	flag.Parse()

	cfg := &Config{
		ListenAddr: *listen,
		DBPath:     *dbPath,
		SecretKey:  *secret,
	}
	if cfg.SecretKey == "" {
		if v := os.Getenv("MIKROTIK_MONITOR_SECRET"); v != "" {
			cfg.SecretKey = v
		}
	}
	return cfg
}

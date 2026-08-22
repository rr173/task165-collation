// Package config centralises runtime configuration for the collation service.
package config

import (
	"flag"
	"fmt"
	"os"
)

// Config holds the runtime options parsed from flags or environment.
type Config struct {
	Addr      string // HTTP listen address
	DBPath    string // SQLite database file
	SmokeTest bool   // run offline self-check and exit
}

// Default returns a Config populated from flag defaults.
func Default() *Config {
	return &Config{Addr: ":8080", DBPath: "task165-collation.db"}
}

// Parse reads flags and environment overrides. Flags win over the
// environment; both win over defaults.
func Parse() (*Config, error) {
	c := Default()
	addr := flag.String("addr", "", "HTTP listen address")
	dbPath := flag.String("db", "", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run offline self-check and exit")
	flag.Parse()
	if *addr != "" {
		c.Addr = *addr
	}
	if *dbPath != "" {
		c.DBPath = *dbPath
	}
	c.SmokeTest = *smoke
	if env := os.Getenv("COLLATION_ADDR"); env != "" && *addr == "" {
		c.Addr = env
	}
	if env := os.Getenv("COLLATION_DB"); env != "" && *dbPath == "" {
		c.DBPath = env
	}
	if c.DBPath == "" {
		return nil, fmt.Errorf("empty database path")
	}
	return c, nil
}

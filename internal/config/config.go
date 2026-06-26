package config

import (
	"crypto/rand"
	"os"
	"strconv"
)

// Config holds application configuration values.
type Config struct {
	Port                  string // HTTP server port, default "8080"
	DBPath                string // SQLite database file path, default "vibes.db"
	IdentitySalt          []byte // Raw salt bytes for identity generation
	IdentityRotationHours int    // Rotation interval in hours (1-8760), default 168
}

// Load reads configuration from environment variables, falling back to defaults.
func Load() Config {
	cfg := Config{
		Port:   "8080",
		DBPath: "vibes.db",
	}

	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}

	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	// Load identity salt from environment or generate random bytes
	if salt := os.Getenv("IDENTITY_SALT"); salt != "" {
		cfg.IdentitySalt = []byte(salt)
	} else {
		cfg.IdentitySalt = make([]byte, 32)
		if _, err := rand.Read(cfg.IdentitySalt); err != nil {
			panic("failed to generate random identity salt: " + err.Error())
		}
	}

	// Load rotation hours with validation
	cfg.IdentityRotationHours = 168
	if hoursStr := os.Getenv("IDENTITY_ROTATION_HOURS"); hoursStr != "" {
		if hours, err := strconv.Atoi(hoursStr); err == nil && hours >= 1 && hours <= 8760 {
			cfg.IdentityRotationHours = hours
		}
	}

	return cfg
}

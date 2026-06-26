package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	// Ensure env vars are unset so defaults apply.
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port %q, got %q", "8080", cfg.Port)
	}
	if cfg.DBPath != "vibes.db" {
		t.Errorf("expected default DBPath %q, got %q", "vibes.db", cfg.DBPath)
	}
}

func TestLoad_EnvOverridePort(t *testing.T) {
	t.Setenv("PORT", "3000")
	t.Setenv("DB_PATH", "")

	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("expected Port %q, got %q", "3000", cfg.Port)
	}
	if cfg.DBPath != "vibes.db" {
		t.Errorf("expected default DBPath %q, got %q", "vibes.db", cfg.DBPath)
	}
}

func TestLoad_EnvOverrideDBPath(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "/tmp/test.db")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port %q, got %q", "8080", cfg.Port)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("expected DBPath %q, got %q", "/tmp/test.db", cfg.DBPath)
	}
}

func TestLoad_EnvOverrideBoth(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_PATH", "/data/my.db")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected Port %q, got %q", "9090", cfg.Port)
	}
	if cfg.DBPath != "/data/my.db" {
		t.Errorf("expected DBPath %q, got %q", "/data/my.db", cfg.DBPath)
	}
}

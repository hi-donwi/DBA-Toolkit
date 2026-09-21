package config

import (
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	clearEnv(t)
	cfg := Load()
	if cfg.Host != DefaultHost || cfg.Port != DefaultPort || cfg.Name != DefaultName ||
		cfg.User != DefaultUser || cfg.SSLMode != DefaultSSLMode {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Password != "" || cfg.URL != "" {
		t.Fatalf("password/url should default empty: %+v", cfg)
	}
}

func TestEnvValues(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvHost, "db.example.net")
	t.Setenv(EnvPort, "5433")
	t.Setenv(EnvName, "analytics")
	t.Setenv(EnvUser, "dba")
	t.Setenv(EnvPassword, "s3cr3t")
	t.Setenv(EnvSSLMode, "require")
	t.Setenv(EnvURL, "postgres://a:b@h/p")

	cfg := Load()
	if cfg.Host != "db.example.net" || cfg.Port != 5433 || cfg.Name != "analytics" ||
		cfg.User != "dba" || cfg.Password != "s3cr3t" || cfg.SSLMode != "require" {
		t.Fatalf("env not applied: %+v", cfg)
	}
	if cfg.URL != "postgres://a:b@h/p" {
		t.Fatalf("url env not applied: %+v", cfg)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvHost, "env-host")
	t.Setenv(EnvPort, "5000")

	host := "flag-host"
	port := 6543
	cfg := Load().WithValues(Values{Host: &host, Port: &port})
	if cfg.Host != "flag-host" || cfg.Port != 6543 {
		t.Fatalf("flag values should win over env: %+v", cfg)
	}

	// Unchanged fields keep env values.
	if cfg.Name != DefaultName {
		t.Fatalf("env-independent field drifted: %+v", cfg)
	}
}

func TestURLWinsOverFields(t *testing.T) {
	cfg := Config{Host: "h", Port: 5432, User: "u", Password: "p", Name: "n", URL: "postgres://u:p@other:5444/d"}
	if got := cfg.ConnectionString(); got != "postgres://u:p@other:5444/d" {
		t.Fatalf("URL should win: %q", got)
	}
}

func TestConnectionStringQuotes(t *testing.T) {
	cfg := Config{Host: "h", Port: 5432, User: "we ird", Password: "p'w", Name: "n"}
	got := cfg.ConnectionString()
	for _, want := range []string{"user='we ird'", "password='p\\'w'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"host=h port=5432 dbname=n user=u password=s3cr3t sslmode=require": "password=***",
		"postgres://alice:hunter2@db:5432/app?sslmode=require":             "postgres://alice:***@db:5432/app?sslmode=require",
		"postgres://db:5432/app":                                           "postgres://db:5432/app",
	}
	for input, want := range cases {
		if got := Redact(input); !strings.Contains(got, want) {
			t.Errorf("Redact(%q) = %q, want to contain %q", input, got, want)
		}
		if strings.Contains(Redact(input), "hunter2") || strings.Contains(Redact(input), "s3cr3t") {
			t.Errorf("Redact(%q) leaked a password", input)
		}
	}
}

func TestRedactedConfigIsSecretFree(t *testing.T) {
	cfg := Config{Host: "db", Port: 5432, User: "u", Name: "n", Password: "topsecretpw", SSLMode: "require"}
	out := cfg.Redacted()
	if strings.Contains(out, "topsecretpw") {
		t.Fatalf("Redacted() leaked the password: %q", out)
	}
	for _, s := range []string{"password=***", "host=db", "dbname=n"} {
		if !strings.Contains(out, s) {
			t.Fatalf("Redacted() missing %q: %q", s, out)
		}
	}
}

func TestInvalidPortIgnored(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPort, "not-a-number")
	if cfg := Load(); cfg.Port != DefaultPort {
		t.Fatalf("invalid port env should keep default, got %d", cfg.Port)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{EnvHost, EnvPort, EnvName, EnvUser, EnvPassword, EnvSSLMode, EnvURL} {
		t.Setenv(name, "") // Load() ignores empty values
	}
}

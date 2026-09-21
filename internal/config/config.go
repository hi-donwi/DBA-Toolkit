// Package config resolves PostgreSQL connection settings from, in descending
// order of precedence: explicit command-line values, DBAKIT_DB_* environment
// variables, a connection URL/DSN, and built-in defaults. It never logs or
// prints the password: every surface that could carry a connection string
// passes through Redact first.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Config is the resolved connection configuration for one dbakit run.
type Config struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
	// URL, when set, is a libpq connection string or URL and overrides every
	// individual field above.
	URL string
}

// Defaults the MVP ships with. Port 5432 and a localhost default match psql's
// behaviour; a deterministic db/user (postgres) avoids surprising connections.
const (
	DefaultHost    = "localhost"
	DefaultPort    = 5432
	DefaultName    = "postgres"
	DefaultUser    = "postgres"
	DefaultSSLMode = "prefer"
)

const EnvPrefix = "DBAKIT_DB_"

// Env names, exported for tests and documentation.
const (
	EnvHost     = EnvPrefix + "HOST"
	EnvPort     = EnvPrefix + "PORT"
	EnvName     = EnvPrefix + "NAME"
	EnvUser     = EnvPrefix + "USER"
	EnvPassword = EnvPrefix + "PASSWORD"
	EnvSSLMode  = EnvPrefix + "SSLMODE"
	EnvURL      = EnvPrefix + "URL"
)

// Load resolves a Config from environment variables and built-in defaults.
// Call WithValues after Load so explicit (flag-provided) values win.
func Load() Config {
	cfg := Config{
		Host:    DefaultHost,
		Port:    DefaultPort,
		Name:    DefaultName,
		User:    DefaultUser,
		SSLMode: DefaultSSLMode,
	}
	if v := strings.TrimSpace(os.Getenv(EnvHost)); v != "" {
		cfg.Host = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvPort)); v != "" {
		if p, err := parsePort(v); err == nil {
			cfg.Port = p
		}
	}
	if v := strings.TrimSpace(os.Getenv(EnvName)); v != "" {
		cfg.Name = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvUser)); v != "" {
		cfg.User = v
	}
	if v := os.Getenv(EnvPassword); v != "" {
		cfg.Password = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvSSLMode)); v != "" {
		cfg.SSLMode = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvURL)); v != "" {
		cfg.URL = v
	}
	return cfg
}

// WithValues overrides cfg fields for every non-nil value. The CLI passes
// pointers only for flags the user actually set, preserving env precedence.
func (c Config) WithValues(v Values) Config {
	if v.Host != nil && *v.Host != "" {
		c.Host = *v.Host
	}
	if v.Port != nil && *v.Port > 0 {
		c.Port = *v.Port
	}
	if v.Name != nil && *v.Name != "" {
		c.Name = *v.Name
	}
	if v.User != nil && *v.User != "" {
		c.User = *v.User
	}
	if v.Password != nil && *v.Password != "" {
		c.Password = *v.Password
	}
	if v.SSLMode != nil && *v.SSLMode != "" {
		c.SSLMode = *v.SSLMode
	}
	if v.URL != nil && *v.URL != "" {
		c.URL = *v.URL
	}
	return c
}

// Values carries explicit (flag) overrides for Config.
type Values struct {
	Host     *string
	Port     *int
	Name     *string
	User     *string
	Password *string
	SSLMode  *string
	URL      *string
}

// ConnectionString returns the libpq connection string or URL to hand to the
// driver. A set URL wins over individual fields. The result may contain the
// password, so never print it directly — use Redacted.
func (c Config) ConnectionString() string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=5",
		c.Host, c.Port, quote(c.Name), quote(c.User), quote(c.Password), c.SSLMode,
	)
}

func quote(s string) string {
	if strings.ContainsAny(s, " '\\") {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `\'`) + "'"
	}
	return s
}

// Redacted renders the connection configuration with the password masked, safe
// for logs and error messages.
func (c Config) Redacted() string {
	if c.URL != "" {
		return "url=" + Redact(c.URL)
	}
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s sslmode=%s password=***",
		c.Host, c.Port, c.Name, c.User, c.SSLMode)
}

func parsePort(s string) (int, error) {
	var p int
	if _, err := fmt.Sscanf(s, "%d", &p); err != nil {
		return 0, err
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("port out of range: %d", p)
	}
	return p, nil
}

var (
	dsnPassword = regexp.MustCompile(`(?i)(password\s*=\s*)'?[^';\s]+'?`)
	urlPassword = regexp.MustCompile(`(://[^:/@\s]+:)[^@\s]+(@)`)
)

// Redact masks the password in any connection string, URL, or error message.
// Use it before the string reaches a report, log, or JSON output.
func Redact(s string) string {
	s = dsnPassword.ReplaceAllString(s, "${1}***")
	s = urlPassword.ReplaceAllString(s, "${1}***${2}")
	return s
}

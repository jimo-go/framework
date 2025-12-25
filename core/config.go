package core

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds framework-level configuration.
//
// It is intentionally small in Phase 1 and will grow as more framework features are added.
type Config struct {
	Env   string
	Debug bool
	Key   string
}

// NewConfig reads configuration from the current process environment.
func NewConfig() *Config {
	cfg := &Config{}
	cfg.RefreshFromEnv()
	return cfg
}

// RefreshFromEnv reloads configuration from the current process environment.
func (c *Config) RefreshFromEnv() {
	if c == nil {
		return
	}

	c.Env = getenvDefault("APP_ENV", "local")
	c.Debug = parseBool(getenvDefault("APP_DEBUG", "true"))
	c.Key = getenvDefault("APP_KEY", "")
}

// LoadEnv loads a .env file and applies variables to the process environment.
//
// Existing environment variables are not overwritten.
func LoadEnv(path string) error {
	vars, err := ParseEnvFile(path)
	if err != nil {
		return err
	}

	for k, v := range vars {
		if _, exists := os.LookupEnv(k); exists {
			continue
		}
		_ = os.Setenv(k, v)
	}
	return nil
}

// AutoLoadEnv attempts to load a .env file from the given directory.
//
// If the file does not exist, it returns nil.
func AutoLoadEnv(dir string) error {
	path := filepath.Join(dir, ".env")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return LoadEnv(path)
}

// ParseEnvFile parses a dotenv-style file.
//
// Supported:
// - KEY=value
// - export KEY=value
// - comments starting with '#'
// - optional single or double quoted values
func ParseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}

		val = unquoteEnv(val)
		vars[key] = val
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

func getenvDefault(key, def string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	return v
}

func parseBool(v string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false
	}
	return b
}

func unquoteEnv(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if (strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"")) || (strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'")) {
		u, err := strconv.Unquote(v)
		if err == nil {
			return u
		}
		// Fallback for single quotes when strconv.Unquote fails.
		return strings.Trim(v, "'\"")
	}
	if strings.ContainsAny(v, "\r\n") {
		return strings.ReplaceAll(strings.ReplaceAll(v, "\r", ""), "\n", "")
	}
	return v
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config: nil")
	}
	return nil
}

// GenerateAppKey returns a base64-encoded application key suitable for sessions/crypto.
func GenerateAppKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "base64:" + base64.StdEncoding.EncodeToString(b), nil
}

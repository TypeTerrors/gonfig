// Package gonfig provides a small helper to load YAML configuration files
// with environment variable interpolation into typed Go structs.
//
// It supports placeholders like ${ENV_VAR} and ${ENV_VAR:-default}, optional
// loading of .env files for local development, and an optional Validate()
// hook on your config struct.
//
// Basic usage:
//
//	type ServerConfig struct {
//	    Port     int    `yaml:"port"`
//	    LogLevel string `yaml:"log_level"`
//	}
//
//	type DatabaseConfig struct {
//	    Host     string `yaml:"host"`
//	    Port     int    `yaml:"port"`
//	    User     string `yaml:"user"`
//	    Password string `yaml:"password"`
//	    Name     string `yaml:"name"`
//	}
//
//	type Config struct {
//	    AppName  string         `yaml:"app_name"`
//	    Env      string         `yaml:"env"`
//	    Server   ServerConfig   `yaml:"server"`
//	    Database DatabaseConfig `yaml:"database"`
//	}
//
//	// Optional: implement Validate() to enforce required fields.
//	func (c Config) Validate() error {
//	    if c.Database.Password == "" {
//	        return fmt.Errorf("database.password is required (set DB_PASSWORD)")
//	    }
//	    return nil
//	}
//
//	func main() {
//	    cfg, err := gonfig.Load[Config](
//	        gonfig.WithConfigFile("config/config.yaml"),
//	        gonfig.WithDotenv(".env.dev"), // optional, for local dev
//	        gonfig.WithStrict(),           // fail if a ${VAR} is missing
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    fmt.Printf("app=%s env=%s port=%d\n",
//	        cfg.AppName, cfg.Env, cfg.Server.Port,
//	    )
//	}
package gonfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type loader struct {
	configFile string
	dotenvs    []string
	strict     bool
}

// Option configures how Load behaves.
//
// Options are passed to Load to customize where configuration is read from.
//
// Example:
//
//	cfg, err := gonfig.Load[Config](
//	    gonfig.WithConfigFile("config/config.yaml"),
//	    gonfig.WithDotenv(".env.dev"),
//	    gonfig.WithStrict(),
//	)
type Option func(*loader)

func defaultLoader() *loader {
	return &loader{
		configFile: "config.yaml",
		dotenvs:    nil,
		strict:     false,
	}
}

// Load reads a YAML config file, expands ${ENV_VAR} placeholders,
// unmarshals into a typed struct T, and optionally calls Validate().
//
// Placeholders:
//
//   - ${VAR}           -> replaced with the value of VAR from the environment
//   - ${VAR:-default}  -> uses VAR if set, otherwise "default"
//
// If strict mode is enabled via WithStrict(), any ${VAR} without a value
// and without a default will cause Load to return an error.
//
// If the target type T implements:
//
//	type Config struct { /* fields */ }
//	func (c Config) Validate() error
//
// then Validate() will be called after unmarshalling, and any error will be
// returned from Load.
//
// Basic example:
//
//	type Config struct {
//	    AppName string `yaml:"app_name"`
//	    Env     string `yaml:"env"`
//	}
//
//	func (c Config) Validate() error {
//	    if c.Env == "" {
//	        return fmt.Errorf("env is required")
//	    }
//	    return nil
//	}
//
//	func main() {
//	    cfg, err := gonfig.Load[Config](
//	        gonfig.WithConfigFile("config.yaml"),
//	        gonfig.WithDotenv(".env.dev"),
//	        gonfig.WithStrict(),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Println(cfg.AppName, cfg.Env)
//	}
func Load[T any](opts ...Option) (T, error) {
	var zero T

	l := defaultLoader()
	for _, opt := range opts {
		opt(l)
	}

	// 1. Load dotenvs (best-effort)
	for _, path := range l.dotenvs {
		if err := loadDotenv(path); err != nil {
			// ignore missing files, fail on other errors
			if !os.IsNotExist(err) {
				return zero, fmt.Errorf("load dotenv %s: %w", path, err)
			}
		}
	}

	// 2. Read YAML file
	raw, err := os.ReadFile(l.configFile)
	if err != nil {
		return zero, fmt.Errorf("read config file %s: %w", l.configFile, err)
	}

	// 3. Expand env placeholders (${VAR}, ${VAR:-default})
	expanded, err := expandEnv(string(raw), l.strict)
	if err != nil {
		return zero, fmt.Errorf("expand env in config: %w", err)
	}

	// 4. Unmarshal YAML into T
	var cfg T
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return zero, fmt.Errorf("unmarshal config yaml: %w", err)
	}

	// 5. If cfg has Validate() error, call it
	if v, ok := any(cfg).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return zero, fmt.Errorf("config validation failed: %w", err)
		}
	}

	return cfg, nil
}

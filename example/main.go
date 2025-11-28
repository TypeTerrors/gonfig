package main

import (
	"fmt"
	"log"
	"os"

	"github.com/TypeTerrors/gonfig"
)

type ServerConfig struct {
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type Config struct {
	AppName  string         `yaml:"app_name"`
	Env      string         `yaml:"env"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
}

// Optional validation hook.
func (c Config) Validate() error {
	if c.AppName == "" {
		return fmt.Errorf("app_name is required")
	}
	if c.Env == "" {
		return fmt.Errorf("env is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("database.password is required (set DB_PASSWORD)")
	}
	return nil
}

func main() {
	// For the example, set some env vars manually:
	os.Setenv("APP_ENV", "dev")
	os.Setenv("DB_PASSWORD", "dev-secret")

	cfg, err := gonfig.Load[Config](
		gonfig.WithConfigFile("example/config.yaml"),
		// gonfig.WithDotenv(".env.dev"), // optional
		gonfig.WithStrict(),
	)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("Loaded config: app=%s env=%s db_host=%s",
		cfg.AppName, cfg.Env, cfg.Database.Host,
	)
}

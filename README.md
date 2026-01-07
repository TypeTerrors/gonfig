# gonfig 🐐

Tiny Go library for **loading YAML config with environment variables into typed structs**.

- Single `config.yaml` as your source of truth  
- Use `${ENV_VAR}` and `${ENV_VAR:-default}` directly in YAML  
- Optional `.env` files for local dev  
- Strong typing via your own Go structs  
- Optional `Validate() error` hook so bad config fails fast  

No frameworks, no templating DSL, no magic. Just **YAML + env → Go struct**.

gonfig aims to be the GOAT of environment-variable–driven config in Go.

---

## Quick example (minimal setup)

**`config.yaml`**:

```yaml
app_name: my-service
env: ${APP_ENV:-dev}

server:
  port: 8080
  log_level: info

database:
  host: ${DB_HOST:-localhost}
  port: 5432
  user: ${DB_USER:-myapp}
  password: ${DB_PASSWORD}
  name: ${DB_NAME:-myapp}
````

**`main.go`**:

```go
package main

import (
    "fmt"
    "log"

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

// Optional: validate required fields
func (c Config) Validate() error {
    if c.Env == "" {
        return fmt.Errorf("env is required")
    }
    if c.Database.Password == "" {
        return fmt.Errorf("database.password is required (set DB_PASSWORD)")
    }
    return nil
}

func main() {
    cfg, err := gonfig.Load[Config](
        gonfig.WithConfigFile("config.yaml"),
        gonfig.WithStrict(), // fail if ${VAR} has no value/default
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("app=%s env=%s port=%d", cfg.AppName, cfg.Env, cfg.Server.Port)
}
```

Run:

```bash
APP_ENV=dev DB_PASSWORD=dev-secret go run .
```

That’s the whole idea.

---

## Why use gonfig?

Typical Go config pain:

* Env vars are flat strings, configs are nested.
* You end up threading `os.Getenv` all over the place.
* You want **sane defaults** in dev but **strict** behaviour in staging/prod.
* You don’t want a heavy “config framework” just to parse a YAML file.

gonfig gives you:

* A **single YAML file** that defines your config shape + defaults.
* Per-environment values and secrets pulled from:

  * real env vars (Docker, K8s, systemd, etc.), and/or
  * optional `.env.<env>` files in dev.
* One call:

  ```go
  cfg, err := gonfig.Load[Config](...)
  ```

and you’re done.

---

## Install

```bash
go get github.com/TypeTerrors/gonfig@v0.1.0
```

(Adjust version/tag as you release new ones.)

Import:

```go
import "github.com/TypeTerrors/gonfig"
```

---

## CLI (optional)

gonfig also ships with a small CLI tool to inspect and work with configs from the terminal.

### Install the CLI

```bash
go install github.com/TypeTerrors/gonfig/cmd/gonfig@latest
```

### Interactive menu

Running `gonfig` with no arguments opens an interactive menu (using the Charmbracelet huh library) where you can:

- **Print a resolved config** (with env expansion, in YAML or JSON)
- **Generate Go structs** from a YAML config file

Example interactive flow:

```text
gonfig
→ asks for config path (default: config/config.yaml)
→ asks for optional .env path
→ choose action:
   - Print resolved config
     → asks for format (yaml/json)
     → asks whether to enable strict mode
   - Generate Go struct from YAML
     → asks for Go package name
     → asks for root struct name
     → asks for optional output file path
```

---

#### Print resolved config (non-interactive)

You can print a resolved config directly from the terminal:

```bash
gonfig print \
  -config config/config.yaml \
  -dotenv .env.dev \
  -format yaml \
  -strict
```

- `-config`: Path to your YAML config file (default: `config.yaml`)
- `-dotenv`: Optional path to a `.env` file to load before expanding placeholders
- `-format`: Output format (`yaml` or `json`, default: `yaml`)
- `-strict`: Enable strict mode (fail if a `${VAR}` is missing and has no default)

---

#### Generate Go structs from YAML

Generate Go struct definitions from a YAML config file:

```bash
gonfig gen-go \
  -config config/config.yaml \
  -pkg config \
  -root Config \
  -o internal/config/config.go
```

You can also ask gonfig to generate a Validate() method based on inline # validate: comments in your YAML:

```bash
gonfig gen-go \
  -config config/config.yaml \
  -pkg config \
  -root Config \
  -o internal/config/config.go \
  -with-validate
```

- `-config`: Path to your YAML config file
- `-pkg`: Go package name for the generated code (e.g. `config`)
- `-root`: Name of the root Go struct type (e.g. `Config`)
- `-o`: Output file path (optional; if omitted, prints to stdout)
- `-with-validate`: If set, also generates a Config.Validate() method based on # validate: comments in your YAML.

Top-level YAML sections are generated as named `*Config` structs (e.g. `ServerConfig`, `DatabaseConfig`) and used from the root struct. Nested objects inside those sections are generated as anonymous structs by default.

---

### Validation from YAML comments

gonfig can read simple validation rules from comments on the same line as a field, using a `# validate:...` prefix.

Supported rules:
- `required`
- `min=<number>`
- `max=<number>`
- `oneof=a|b|c` (values separated by `|`)

Example YAML:

```yaml
server:
  port: 8080      # validate:required,min=1,max=65535
  log_level: info # validate:required,oneof=debug|info|warn|error

database:
  password: ${DB_PASSWORD} # validate:required
```

When you run `gonfig gen-go` with `-with-validate`, these comments are turned into a `Validate() error` method on your root struct, e.g. `func (c Config) Validate() error`. The `Validate()` method is called automatically by `gonfig.Load` as shown earlier in the README.

---

## Recommended layout for real services

A realistic layout for a Go service using gonfig:

```text
your-service/
  cmd/api/main.go
  internal/config/config.go
  config/config.yaml
  .env.dev           # optional, for local dev only
  .env.stage         # optional
  .env.prod          # optional
```

### `config/config.yaml`

A richer example with server, DB, Redis, HTTP client, and feature flags:

```yaml
app_name: my-service
env: ${APP_ENV:-dev}

server:
  port: 8080
  log_level: info
  read_timeout_seconds: 15
  write_timeout_seconds: 15

database:
  host: ${DB_HOST:-localhost}
  port: 5432
  user: ${DB_USER:-myapp}
  password: ${DB_PASSWORD}
  name: ${DB_NAME:-myapp}
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime_seconds: 300

redis:
  enabled: true
  addr: ${REDIS_ADDR:-localhost:6379}
  db: 0
  password: ${REDIS_PASSWORD:-}

http_client:
  timeout_seconds: 10
  max_idle_conns: 100
  max_idle_conns_per_host: 10
  idle_conn_timeout_seconds: 90

features:
  enable_signup: true
  enable_metrics: true
  enable_request_logging: true
  log_sql_queries: false
```

### `internal/config/config.go`

This package wraps gonfig so the rest of your app doesn’t care how config is loaded:

```go
package config

import (
    "fmt"
    "os"

    "github.com/TypeTerrors/gonfig"
)

type Server struct {
    Port                int    `yaml:"port"`
    LogLevel            string `yaml:"log_level"`
    ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
    WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

type Database struct {
    Host                   string `yaml:"host"`
    Port                   int    `yaml:"port"`
    User                   string `yaml:"user"`
    Password               string `yaml:"password"`
    Name                   string `yaml:"name"`
    MaxOpenConns           int    `yaml:"max_open_conns"`
    MaxIdleConns           int    `yaml:"max_idle_conns"`
    ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

type Redis struct {
    Enabled  bool   `yaml:"enabled"`
    Addr     string `yaml:"addr"`
    DB       int    `yaml:"db"`
    Password string `yaml:"password"`
}

type HTTPClient struct {
    TimeoutSeconds         int `yaml:"timeout_seconds"`
    MaxIdleConns           int `yaml:"max_idle_conns"`
    MaxIdleConnsPerHost    int `yaml:"max_idle_conns_per_host"`
    IdleConnTimeoutSeconds int `yaml:"idle_conn_timeout_seconds"`
}

type Features struct {
    EnableSignup         bool `yaml:"enable_signup"`
    EnableMetrics        bool `yaml:"enable_metrics"`
    EnableRequestLogging bool `yaml:"enable_request_logging"`
    LogSQLQueries        bool `yaml:"log_sql_queries"`
}

type Config struct {
    AppName    string     `yaml:"app_name"`
    Env        string     `yaml:"env"`
    Server     Server     `yaml:"server"`
    Database   Database   `yaml:"database"`
    Redis      Redis      `yaml:"redis"`
    HTTPClient HTTPClient `yaml:"http_client"`
    Features   Features   `yaml:"features"`
}

// Validate enforces "this config is sane enough to run".
func (c Config) Validate() error {
    if c.AppName == "" {
        return fmt.Errorf("app_name is required")
    }

    if c.Env == "" {
        return fmt.Errorf("env is required (set APP_ENV)")
    }

    if c.Server.Port <= 0 || c.Server.Port > 65535 {
        return fmt.Errorf("server.port must be between 1 and 65535")
    }

    switch c.Server.LogLevel {
    case "debug", "info", "warn", "error":
        // ok
    default:
        return fmt.Errorf("server.log_level must be one of: debug, info, warn, error")
    }

    if c.Database.Host == "" {
        return fmt.Errorf("database.host is required")
    }
    if c.Database.User == "" {
        return fmt.Errorf("database.user is required")
    }
    if c.Database.Password == "" {
        return fmt.Errorf("database.password is required (set DB_PASSWORD)")
    }

    if c.HTTPClient.TimeoutSeconds <= 0 {
        return fmt.Errorf("http_client.timeout_seconds must be > 0")
    }

    if c.Redis.Enabled && c.Redis.Addr == "" {
        return fmt.Errorf("redis.enabled=true but redis.addr is empty (set REDIS_ADDR)")
    }

    return nil
}

// Load reads config/config.yaml, applies .env.<env>, and returns Config.
//
// APP_ENV controls which .env file to try:
//   APP_ENV=dev   -> .env.dev
//   APP_ENV=stage -> .env.stage
//   APP_ENV=prod  -> .env.prod
//
// In production you can skip .env files entirely and just rely on real env vars.
func Load() (Config, error) {
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = "dev"
    }

    dotenvFile := ".env." + env

    return gonfig.Load[Config](
        gonfig.WithConfigFile("config/config.yaml"),
        gonfig.WithDotenv(dotenvFile), // ignored if missing
        gonfig.WithStrict(),           // fail if required envs are missing
    )
}
```

### `cmd/api/main.go`

Your entrypoint just asks `internal/config` for a ready-to-use `Config` and wires things up:

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "your-service/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    log.Printf("starting %s in %s on port %d",
        cfg.AppName, cfg.Env, cfg.Server.Port,
    )

    // Example: build shared HTTP client from config.
    httpClient := &http.Client{
        Timeout: time.Duration(cfg.HTTPClient.TimeoutSeconds) * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        cfg.HTTPClient.MaxIdleConns,
            MaxIdleConnsPerHost: cfg.HTTPClient.MaxIdleConnsPerHost,
            IdleConnTimeout:     time.Duration(cfg.HTTPClient.IdleConnTimeoutSeconds) * time.Second,
        },
    }
    _ = httpClient // pass into downstream services

    // Example: use feature flags.
    if cfg.Features.EnableMetrics {
        log.Println("metrics enabled")
        // init metrics pipeline here
    }

    if cfg.Features.EnableRequestLogging {
        log.Println("request logging enabled")
        // wrap handlers with logging middleware here
    }

    // TODO: init DB with cfg.Database
    // TODO: init Redis if cfg.Redis.Enabled

    srv := &http.Server{
        Addr:         ":" + fmt.Sprint(cfg.Server.Port),
        ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
        WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
        Handler:      nil, // your mux/router
    }

    log.Fatal(srv.ListenAndServe())
}
```

You can trim or expand this in your own services, but this gives a realistic pattern.

---

## Placeholder syntax

gonfig supports two forms inside YAML:

* `${VAR}`
  → replaced with the value of env var `VAR`
  → in strict mode, missing `VAR` causes an error

* `${VAR:-default}`
  → uses env var `VAR` if set, otherwise the literal `"default"`

Examples:

```yaml
env: ${APP_ENV:-dev}

database:
  host: ${DB_HOST:-localhost}
  password: ${DB_PASSWORD}        # must be set if strict mode enabled
```

---

## API overview (v1)

### `Load[T any](opts ...Option) (T, error)`

Core entrypoint.

* Reads the YAML file (default: `config.yaml`)
* Expands `${VAR}` and `${VAR:-default}`
* Unmarshals into `T`
* If `T` has `Validate() error`, calls it
* Returns `T` or an error

```go
cfg, err := gonfig.Load[Config](
    gonfig.WithConfigFile("config/config.yaml"),
    gonfig.WithDotenv(".env.dev"),
    gonfig.WithStrict(),
)
```

### `WithConfigFile(path string) Option`

Set a custom path to your YAML config.

```go
gonfig.Load[Config](
    gonfig.WithConfigFile("config/config.yaml"),
)
```

### `WithDotenv(path string) Option`

Load variables from a `.env` file into the process environment **before** expanding placeholders.

* Missing `.env` files are ignored.
* Great for dev; prod should use real env vars.

```go
gonfig.Load[Config](
    gonfig.WithDotenv(".env.dev"),
)
```

You can pass multiple `.env` files if you want layering.

### `WithStrict() Option`

Enable strict mode for env expansion:

* `${VAR}` without a value and without a default → error
* `${VAR:-default}` is always safe

```go
gonfig.Load[Config](
    gonfig.WithConfigFile("config.yaml"),
    gonfig.WithStrict(),
)
```

---

## How it works under the hood

gonfig is intentionally small:

1. **Load `.env` files (optional)**
   If you use `WithDotenv`, those key/values are loaded into the process environment.

2. **Read YAML as raw text**
   Your config file is read into a string.

3. **Expand `${VAR}` and `${VAR:-default}`**
   The text is scanned and placeholders are replaced using `os.LookupEnv`.
   In strict mode, missing `${VAR}` without a default causes an error.

4. **Unmarshal into your struct**
   Expanded YAML is parsed using `gopkg.in/yaml.v3`.

5. **Validation hook**
   If your type implements `Validate() error`, it’s called, and any error is returned.

No hidden globals beyond the process env. No runtime magic beyond YAML’s usual reflection.

---

## Example project

See the [`example/`](./example) directory in this repo for a small, runnable example using:

* `example/config.yaml`
* `example/main.go`

You can copy that structure or use the `cmd/api` + `internal/config` pattern shown above.

---

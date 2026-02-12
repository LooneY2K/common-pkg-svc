# common-pkg-svc

Production-ready Go utilities for configuration, JSON handling, type conversion, and structured logging.

[![Go Reference](https://pkg.go.dev/badge/github.com/LooneY2K/common-pkg-svc.svg)](https://pkg.go.dev/github.com/LooneY2K/common-pkg-svc)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

## Installation

```bash
go get github.com/LooneY2K/common-pkg-svc
```

## Packages

| Package | Description |
|---------|-------------|
| [config](https://pkg.go.dev/github.com/LooneY2K/common-pkg-svc/config) | JSON config loading, env overrides, dot-notation access |
| [json](https://pkg.go.dev/github.com/LooneY2K/common-pkg-svc/json) | JSON file loading and unmarshaling into maps |
| [converter](https://pkg.go.dev/github.com/LooneY2K/common-pkg-svc/converter) | Type conversion (string, int, bool, duration, etc.) |
| [log](https://pkg.go.dev/github.com/LooneY2K/common-pkg-svc/log) | Structured logger with pretty/JSON modes and levels |

---

## Usage

### Config

Load configuration from a JSON file with environment variable overrides and type-safe accessors:

```go
import (
    "github.com/LooneY2K/common-pkg-svc/config"
)

// Load from file
cfg, err := config.Load("config.json", config.WithEnvPrefix("APP"))
if err != nil {
    log.Fatal(err)
}

// Dot-notation access
level := cfg.GetString("log.level")       // "info"
port := cfg.GetInt("server.port")         // 8080
timeout := cfg.GetDuration("server.timeout") // 30s
ssl := cfg.GetBool("database.ssl")        // true

// With defaults
host := cfg.GetStringOrDefault("database.host", "localhost")

// Unmarshal nested sections into structs
var db struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}
cfg.UnmarshalKey("database", &db)
```

**Example `config.json`:**

```json
{
  "log": {"level": "info", "mode": "json", "component": "my-service"},
  "server": {"port": 8080, "timeout": "30s"},
  "database": {"host": "localhost", "port": 5432, "ssl": true}
}
```

Environment overrides (with `WithEnvPrefix("APP")`): `APP_LOG_LEVEL`, `APP_SERVER_PORT`, etc.

---

### JSON

Load and unmarshal JSON files into `map[string]any` with dot-path support:

```go
import cfgjson "github.com/LooneY2K/common-pkg-svc/json"

// Load from file
m, err := cfgjson.Load("config.json")
if err != nil {
    return err
}

// Unmarshal raw bytes
m, err = cfgjson.Unmarshal([]byte(`{"a":1,"b":"two"}`))

// Unmarshal subset into struct (dot path)
var db struct { Host string `json:"host"`; Port int `json:"port"` }
err = cfgjson.UnmarshalInto(m, "database", &db)
```

> Use an import alias (`cfgjson`) to avoid conflicts with `encoding/json`.

---

### Converter

Convert values between types safely:

```go
import "github.com/LooneY2K/common-pkg-svc/converter"

converter.ToString(42)           // "42"
converter.ToInt("42")            // 42
converter.ToInt64(42)            // int64(42)
converter.ToBool("true")         // true
converter.ToFloat64("3.14")      // 3.14
converter.ToDuration("5s")       // 5*time.Second
```

---

### Log

Structured logger with Pretty and JSON modes, level filtering, and structured fields:

```go
import "github.com/LooneY2K/common-pkg-svc/log"

logger := log.New(
    log.WithLevel(log.Debug),
    log.WithComponent("api"),
    log.WithMode(log.JSON),      // or log.Pretty
)

logger.Info("server started", log.String("port", "8080"))
logger.Debug("processing", log.Int("count", 10), log.Duration("elapsed", elapsed))
logger.Error("failed", log.Err(err))
```

---

## Testing

### Run all tests

```bash
go test ./...
```

### Run tests in the tests package

```bash
go test ./tests/ -v
```

### Run specific tests

```bash
# Config
go test ./tests/ -v -run TestConfig_

# Converter
go test ./tests/ -v -run TestConverter_

# JSON
go test ./tests/ -v -run TestJson_

# Log (includes benchmarks)
go test ./tests/ -v -run TestLogger_
go test ./tests/ -bench=. -benchmem
```

### Interactive test runner

The project includes an interactive test runner for exploratory testing:

```bash
make tests
# or
go run ./tests/
```

This launches a prompt to select a test group (config, converter, json, log) and a specific test to run.

---

## License

See [LICENSE](LICENSE) for details.

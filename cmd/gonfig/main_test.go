package main

import (
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateGoCode_TopLevelSectionsBecomeNamedTypes(t *testing.T) {
	m := map[string]any{
		"app_name": "my-service",
		"server": map[string]any{
			"port":      8080,
			"log_level": "info",
		},
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
		},
	}

	code := generateGoCode("config", "Config", m, nil)

	if !strings.Contains(code, "type ServerConfig struct") {
		t.Fatalf("expected named ServerConfig struct to be generated")
	}
	if !strings.Contains(code, "type DatabaseConfig struct") {
		t.Fatalf("expected named DatabaseConfig struct to be generated")
	}
	if !strings.Contains(code, "Server ServerConfig `yaml:\"server\"`") {
		t.Fatalf("expected root Config to reference ServerConfig")
	}
	if strings.Contains(code, "Server struct {") {
		t.Fatalf("did not expect anonymous top-level struct for server section")
	}

	assertGeneratedGoParses(t, code)
}

func TestGenerateGoCode_WithValidateAddsFmtImport(t *testing.T) {
	m := map[string]any{
		"app_name": "my-service",
	}
	validations := []fieldValidation{
		{
			GoExpr:   "c.AppName",
			YAMLPath: "app_name",
			GoType:   "string",
			Required: true,
		},
	}

	code := generateGoCode("config", "Config", m, validations)

	if !strings.Contains(code, "import \"fmt\"") {
		t.Fatalf("expected fmt to be imported when Validate() is generated (single import form)")
	}
	if !strings.Contains(code, "func (c Config) Validate() error") {
		t.Fatalf("expected Validate() method to be generated")
	}

	assertGeneratedGoParses(t, code)
}

func assertGeneratedGoParses(t *testing.T, code string) {
	t.Helper()

	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("gofmt failed on generated code: %v\n\n%s", err, code)
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "generated.go", formatted, parser.AllErrors); err != nil {
		t.Fatalf("failed to parse generated code: %v\n\n%s", err, string(formatted))
	}
}

// expand.go
package gonfig

import (
    "fmt"
    "os"
    "regexp"
    "strings"
)

var rePlaceholder = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnv replaces ${VAR} or ${VAR:-default} with env values.
// strict=true: missing env without default -> error.
func expandEnv(s string, strict bool) (string, error) {
    var missing []string

    out := rePlaceholder.ReplaceAllStringFunc(s, func(m string) string {
        matches := rePlaceholder.FindStringSubmatch(m)
        if len(matches) != 2 {
            // shouldn't happen, but be defensive
            return m
        }
        inner := matches[1]

        name := inner
        var def *string

        // Support syntax: VAR:-default
        if idx := strings.Index(inner, ":-"); idx != -1 {
            n := inner[:idx]
            d := inner[idx+2:]
            name = n
            def = &d
        }

        if val, ok := os.LookupEnv(name); ok {
            return val
        }

        if def != nil {
            return *def
        }

        if strict {
            missing = append(missing, name)
        }

        // non-strict: replace with empty string
        return ""
    })

    if len(missing) > 0 {
        return "", fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
    }

    return out, nil
}
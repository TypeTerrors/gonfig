package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go/format"

	"gopkg.in/yaml.v3"

	"github.com/TypeTerrors/gonfig"
	"github.com/charmbracelet/huh"
)

// main is the entry point for the gonfig CLI. It supports both an
// interactive mode powered by the Charmbracelet huh library as well as
// traditional subcommands. If no subcommand is provided, an interactive
// menu will be shown by default.
func main() {
	// When run without arguments (or with unrecognized subcommand) we drop
	// into an interactive menu. Otherwise we dispatch based on the first
	// argument. The "interactive" or "menu" subcommand can also force
	// interactive mode explicitly.
	if len(os.Args) <= 1 {
		runInteractive()
		return
	}
	sub := os.Args[1]
	switch sub {
	case "print":
		runPrint(os.Args[2:])
	case "gen-go":
		runGenGo(os.Args[2:])
	case "interactive", "menu":
		runInteractive()
	default:
		// Unknown subcommand. Assume interactive to avoid memorizing flags.
		runInteractive()
	}
}

// runInteractive presents a menu using the huh library to collect user
// input. Based on the selected action it delegates to runPrint or runGenGo
// with synthesized arguments. If huh cannot run (e.g. no TTY), the program
// will fall back to the print subcommand with sensible defaults.
func runInteractive() {
	// If we’re in a non-interactive environment (no TTY), just run print with
	// defaults. We check if stdin is a terminal. If not, default to print.
	if !isTerminal(os.Stdin.Fd()) {
		runPrint([]string{})
		return
	}

	// Select action
	var action string
	sel := huh.NewSelect[string]().
		Title("What would you like to do?").
		Options(
			huh.NewOption("Print resolved config", "print"),
			huh.NewOption("Generate Go struct from YAML", "gen-go"),
		).
		Value(&action)
	if err := sel.Run(); err != nil {
		log.Fatalf("menu error: %v", err)
	}

	// Ask for the config file path. Provide a default based on common
	// convention.
	var configPath string = "config/config.yaml"
	cfgInput := huh.NewInput().
		Title("Path to YAML config file").
		Value(&configPath)
	if err := cfgInput.Run(); err != nil {
		log.Fatalf("failed to read config path: %v", err)
	}

	// For printing the config we may need a .env file and output format. Ask
	// early so we can reuse responses. If the user chooses gen-go the .env
	// value is ignored.
	var dotenv string
	dotenvInput := huh.NewInput().
		Title("Path to .env file (optional)").
		Value(&dotenv)
	if err := dotenvInput.Run(); err != nil {
		log.Fatalf("failed to read dotenv path: %v", err)
	}

	switch action {
	case "print":
		// Output format
		var format string = "yaml"
		formatSel := huh.NewSelect[string]().
			Title("Output format").
			Options(
				huh.NewOption("YAML", "yaml"),
				huh.NewOption("JSON", "json"),
			).
			Value(&format)
		if err := formatSel.Run(); err != nil {
			log.Fatalf("failed to choose format: %v", err)
		}
		// Strict mode
		var strict bool
		strictConfirm := huh.NewConfirm().
			Title("Enable strict mode?").
			Value(&strict)
		if err := strictConfirm.Run(); err != nil {
			log.Fatalf("failed to choose strict mode: %v", err)
		}
		// Build args for runPrint
		args := []string{"-config", configPath}
		if dotenv != "" {
			args = append(args, "-dotenv", dotenv)
		}
		args = append(args, "-format", format)
		if strict {
			args = append(args, "-strict")
		}
		runPrint(args)
	case "gen-go":
		// Package name
		var pkgName string = "config"
		pkgInput := huh.NewInput().
			Title("Go package name for generated code").
			Value(&pkgName)
		if err := pkgInput.Run(); err != nil {
			log.Fatalf("failed to read package name: %v", err)
		}
		// Root struct name
		var rootName string = "Config"
		rootInput := huh.NewInput().
			Title("Name of root Go struct").
			Value(&rootName)
		if err := rootInput.Run(); err != nil {
			log.Fatalf("failed to read root struct name: %v", err)
		}
		// Output file path (optional)
		var outPath string
		outInput := huh.NewInput().
			Title("Output file path (optional, leave blank to print)").
			Value(&outPath)
		if err := outInput.Run(); err != nil {
			log.Fatalf("failed to read output path: %v", err)
		}
		// Ask if we want Validate() method
		var withValidate bool = false
		validateConfirm := huh.NewConfirm().
			Title("Generate Validate() from # validate: comments?").
			Value(&withValidate)
		if err := validateConfirm.Run(); err != nil {
			log.Fatalf("failed to choose validate method: %v", err)
		}
		// Build args for runGenGo
		args := []string{"-config", configPath, "-pkg", pkgName, "-root", rootName}
		if outPath != "" {
			args = append(args, "-o", outPath)
		}
		if withValidate {
			args = append(args, "-with-validate")
		}
		runGenGo(args)
	}
}

// isTerminal reports whether the given file descriptor is a terminal. We use
// this to detect non-interactive environments and fall back gracefully.
func isTerminal(fd uintptr) bool {
	// We use the GOOS-specific implementation from golang.org/x/crypto/ssh/terminal
	// to check for a terminal. If it’s unavailable we default to true.
	// Because we don't import an additional dependency here, we approximate
	// by checking the TERM environment variable. This heuristic works for
	// most scenarios and avoids pulling in more deps.
	if term := os.Getenv("TERM"); term == "" || term == "dumb" {
		return false
	}
	return true
}

// runPrint implements the "print" subcommand. It resolves the config using
// gonfig and prints the result in YAML or JSON. It expects flag-style args.
func runPrint(args []string) {
	fs := flag.NewFlagSet("print", flag.ExitOnError)
	var (
		configPath string
		dotenvPath string
		format     string
		strict     bool
	)
	fs.StringVar(&configPath, "config", "config.yaml", "Path to YAML config file")
	fs.StringVar(&dotenvPath, "dotenv", "", "Optional .env file to load before parsing config")
	fs.StringVar(&format, "format", "yaml", "Output format: yaml or json")
	fs.BoolVar(&strict, "strict", false, "Enable strict mode (missing ${VAR} without default -> error)")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	opts := []gonfig.Option{gonfig.WithConfigFile(configPath)}
	if dotenvPath != "" {
		opts = append(opts, gonfig.WithDotenv(dotenvPath))
	}
	if strict {
		opts = append(opts, gonfig.WithStrict())
	}
	cfg, err := gonfig.Load[map[string]any](opts...)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	switch format {
	case "yaml", "yml":
		out, err := yaml.Marshal(cfg)
		if err != nil {
			log.Fatalf("failed to marshal config to YAML: %v", err)
		}
		fmt.Println(string(out))
	case "json":
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal config to JSON: %v", err)
		}
		fmt.Println(string(out))
	default:
		log.Fatalf("unknown format %q (expected yaml or json)", format)
	}
}

// runGenGo implements the "gen-go" subcommand. It parses the YAML config
// structure and emits a Go struct definition. It expects flag-style args.
func runGenGo(args []string) {
	fs := flag.NewFlagSet("gen-go", flag.ExitOnError)
	var (
		configPath   string
		pkgName      string
		rootName     string
		outPath      string
		withValidate bool
	)
	fs.StringVar(&configPath, "config", "config.yaml", "Path to YAML config file")
	fs.StringVar(&pkgName, "pkg", "config", "Go package name for generated code")
	fs.StringVar(&rootName, "root", "Config", "Name of root Go struct type")
	fs.StringVar(&outPath, "o", "", "Output file (default: stdout)")
	fs.BoolVar(&withValidate, "with-validate", false, "Generate Validate() method based on # validate: comments")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config file %s: %v", configPath, err)
	}
	// Unmarshal into yaml.Node for AST access (comments)
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		log.Fatalf("failed to parse YAML (AST): %v", err)
	}
	var data any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		log.Fatalf("failed to parse YAML: %v", err)
	}
	m, ok := data.(map[string]any)
	if !ok {
		log.Fatalf("expected top-level YAML mapping (object), got %T", data)
	}
	var validations []fieldValidation
	if withValidate {
		validations = collectValidations(&root, rootName)
	}
	code := generateGoCode(pkgName, rootName, m, validations)
	formatted, err := format.Source([]byte(code))
	if err != nil {
		// If gofmt fails, still output unformatted code so user can see it.
		log.Printf("warning: gofmt failed: %v (printing unformatted code)", err)
		formatted = []byte(code)
	}
	if outPath == "" {
		fmt.Print(string(formatted))
		return
	}
	if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
		log.Fatalf("failed to write output file %s: %v", outPath, err)
	}
	log.Printf("generated Go config struct at %s", outPath)
}

// generateGoCode builds Go code for a struct type representing the given YAML
// mapping. It uses anonymous structs for nested objects. If validations are provided, emits Validate().
func generateGoCode(pkgName, rootName string, m map[string]any, validations []fieldValidation) string {
	var b strings.Builder
	b.WriteString("// Code generated by gonfig gen-go; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkgName)

	imports := requiredImports(validations)
	if len(imports) > 0 {
		if len(imports) == 1 {
			fmt.Fprintf(&b, "import %q\n\n", imports[0])
		} else {
			b.WriteString("import (\n")
			for _, imp := range imports {
				fmt.Fprintf(&b, "    %q\n", imp)
			}
			b.WriteString(")\n\n")
		}
	}

	topLevelTypes := topLevelNamedStructs(rootName, m)
	if len(topLevelTypes) > 0 {
		keys := sortedKeys(m)
		for _, key := range keys {
			typeName, ok := topLevelTypes[key]
			if !ok {
				continue
			}
			sectionMap, _ := m[key].(map[string]any)
			writeStruct(&b, typeName, sectionMap, 0)
			b.WriteString("\n\n")
		}
	}

	writeRootStruct(&b, rootName, m, topLevelTypes)
	if len(validations) > 0 {
		b.WriteString("\n\n")
		writeValidateMethod(&b, rootName, validations)
	}
	return b.String()
}

func writeStruct(b *strings.Builder, name string, m map[string]any, indent int) {
	indentStr := strings.Repeat("    ", indent)
	fmt.Fprintf(b, "%stype %s struct {\n", indentStr, name)
	keys := sortedKeys(m)
	for _, key := range keys {
		val := m[key]
		fieldName := toExportedName(key)
		typeExpr := goTypeExpr(val, indent+1)
		fieldIndent := strings.Repeat("    ", indent+1)
		fmt.Fprintf(b, "%s%s %s `yaml:\"%s\"`\n", fieldIndent, fieldName, typeExpr, key)
	}
	fmt.Fprintf(b, "%s}\n", indentStr)
}

func writeRootStruct(b *strings.Builder, name string, m map[string]any, topLevelTypes map[string]string) {
	fmt.Fprintf(b, "type %s struct {\n", name)
	keys := sortedKeys(m)
	for _, key := range keys {
		val := m[key]
		fieldName := toExportedName(key)
		typeExpr := goTypeExpr(val, 1)
		if named, ok := topLevelTypes[key]; ok {
			typeExpr = named
		}
		fmt.Fprintf(b, "    %s %s `yaml:\"%s\"`\n", fieldName, typeExpr, key)
	}
	b.WriteString("}\n")
}

func requiredImports(validations []fieldValidation) []string {
	if len(validations) == 0 {
		return nil
	}
	// Validate() uses fmt.Errorf.
	return []string{"fmt"}
}

func topLevelNamedStructs(rootName string, m map[string]any) map[string]string {
	keys := sortedKeys(m)

	used := map[string]bool{rootName: true}
	out := make(map[string]string)
	for _, key := range keys {
		if _, ok := m[key].(map[string]any); !ok {
			continue
		}
		base := toExportedName(key)
		if base == "" {
			base = "Section"
		}
		name := base + "Config"
		if name == rootName {
			name = name + "Section"
		}
		if used[name] {
			i := 2
			for used[name+strconv.Itoa(i)] {
				i++
			}
			name = name + strconv.Itoa(i)
		}
		used[name] = true
		out[key] = name
	}
	return out
}

// goTypeExpr returns a Go type expression for the given YAML value.
// For nested maps it returns an anonymous struct type. For lists it uses the
// first element to infer element type.
func goTypeExpr(v any, indent int) string {
	switch v := v.(type) {
	case map[string]any:
		return anonymousStructType(v, indent)
	case []any:
		if len(v) == 0 {
			return "[]any"
		}
		elemType := goTypeExpr(v[0], indent)
		return "[]" + elemType
	case bool:
		return "bool"
	case int, int8, int16, int32, int64:
		return "int"
	case float32, float64:
		return "float64"
	case string:
		return "string"
	default:
		return "any"
	}
}

// anonymousStructType builds an anonymous struct type expression for a nested
// mapping. It recurses on nested maps and lists.
func anonymousStructType(m map[string]any, indent int) string {
	var b strings.Builder
	indentStr := strings.Repeat("    ", indent)
	b.WriteString("struct {\n")
	keys := sortedKeys(m)
	for _, key := range keys {
		val := m[key]
		fieldName := toExportedName(key)
		typeExpr := goTypeExpr(val, indent+1)
		fieldIndent := strings.Repeat("    ", indent+1)
		fmt.Fprintf(&b, "%s%s %s `yaml:\"%s\"`\n", fieldIndent, fieldName, typeExpr, key)
	}
	fmt.Fprintf(&b, "%s}", indentStr)
	return b.String()
}

// sortedKeys returns the keys of m sorted lexicographically.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// toExportedName converts a YAML key like "app_name" or "http-client" into
// an exported Go field name like "AppName" or "HttpClient". It splits on
// underscores, hyphens, spaces and dots.
func toExportedName(key string) string {
	// Split on common separators.
	splitFn := func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	}
	parts := strings.FieldsFunc(key, splitFn)
	if len(parts) == 0 {
		return "Field"
	}
	for i, p := range parts {
		if p == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(p)
		if r == utf8.RuneError {
			continue
		}
		parts[i] = string(unicode.ToUpper(r)) + p[size:]
	}
	name := strings.Join(parts, "")
	// Ensure first rune is exported.
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError {
		return name
	}
	if unicode.IsLower(r) {
		name = string(unicode.ToUpper(r)) + name[size:]
	}
	if name == "" {
		return "Field"
	}
	return name
}

// --- Validation helpers and types ---

type fieldValidation struct {
	GoExpr   string
	YAMLPath string
	GoType   string
	Required bool
	Min      *float64
	Max      *float64
	OneOf    []string
}

type validateRules struct {
	Required bool
	Min      *float64
	Max      *float64
	OneOf    []string
}

// collectValidations walks the yaml.Node AST and collects validation rules from comments.
func collectValidations(root *yaml.Node, rootName string) []fieldValidation {
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return walkMappingValidations(root.Content[0], "", "c")
	}
	return nil
}

func walkMappingValidations(node *yaml.Node, yamlPathPrefix, goExprPrefix string) []fieldValidation {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	var vals []fieldValidation
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value
		yamlPath := key
		if yamlPathPrefix != "" {
			yamlPath = yamlPathPrefix + "." + key
		}
		fieldName := toExportedName(key)
		goExpr := goExprPrefix + "." + fieldName
		// Parse validation from LineComment
		rules, ok := parseValidateComment(valNode.LineComment)
		if ok {
			goType := inferGoTypeFromNode(valNode)
			vals = append(vals, fieldValidation{
				GoExpr:   goExpr,
				YAMLPath: yamlPath,
				GoType:   goType,
				Required: rules.Required,
				Min:      rules.Min,
				Max:      rules.Max,
				OneOf:    rules.OneOf,
			})
		}
		// Recurse into mappings
		if valNode.Kind == yaml.MappingNode {
			child := walkMappingValidations(valNode, yamlPath, goExpr)
			vals = append(vals, child...)
		}
	}
	return vals
}

func inferGoTypeFromNode(n *yaml.Node) string {
	if n.Kind != yaml.ScalarNode {
		return "any"
	}
	switch n.Tag {
	case "!!bool":
		return "bool"
	case "!!int":
		return "int"
	case "!!float":
		return "float64"
	default:
		return "string"
	}
}

func parseValidateComment(comment string) (validateRules, bool) {
	var rules validateRules
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return rules, false
	}
	if strings.HasPrefix(comment, "#") {
		comment = strings.TrimSpace(comment[1:])
	}
	if !strings.HasPrefix(comment, "validate:") {
		return rules, false
	}
	body := strings.TrimSpace(comment[len("validate:"):])
	if body == "" {
		return rules, false
	}
	parts := strings.Split(body, ",")
	found := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch {
		case part == "required":
			rules.Required = true
			found = true
		case strings.HasPrefix(part, "min="):
			val := strings.TrimSpace(part[len("min="):])
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				rules.Min = new(float64)
				*rules.Min = f
				found = true
			}
		case strings.HasPrefix(part, "max="):
			val := strings.TrimSpace(part[len("max="):])
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				rules.Max = new(float64)
				*rules.Max = f
				found = true
			}
		case strings.HasPrefix(part, "oneof="):
			val := part[len("oneof="):]
			opts := strings.Split(val, "|")
			var filtered []string
			for _, o := range opts {
				o = strings.TrimSpace(o)
				if o != "" {
					filtered = append(filtered, o)
				}
			}
			if len(filtered) > 0 {
				rules.OneOf = filtered
				found = true
			}
		}
	}
	return rules, found
}

func writeValidateMethod(b *strings.Builder, rootName string, vals []fieldValidation) {
	fmt.Fprintf(b, "func (c %s) Validate() error {\n", rootName)
	for _, v := range vals {
		// Required
		if v.Required {
			switch v.GoType {
			case "string":
				fmt.Fprintf(b, "    if %s == \"\" {\n        return fmt.Errorf(\"%s is required\")\n    }\n", v.GoExpr, v.YAMLPath)
			case "int", "float64":
				fmt.Fprintf(b, "    if %s == 0 {\n        return fmt.Errorf(\"%s is required\")\n    }\n", v.GoExpr, v.YAMLPath)
			}
		}
		// Min/Max
		if (v.Min != nil || v.Max != nil) && (v.GoType == "int" || v.GoType == "float64") {
			if v.Min != nil {
				fmt.Fprintf(b, "    if %s < %v {\n        return fmt.Errorf(\"%s must be >= %v\")\n    }\n", v.GoExpr, *v.Min, v.YAMLPath, *v.Min)
			}
			if v.Max != nil {
				fmt.Fprintf(b, "    if %s > %v {\n        return fmt.Errorf(\"%s must be <= %v\")\n    }\n", v.GoExpr, *v.Max, v.YAMLPath, *v.Max)
			}
		}
		// OneOf
		if len(v.OneOf) > 0 && v.GoType == "string" {
			fmt.Fprintf(b, "    switch %s {\n", v.GoExpr)
			for _, opt := range v.OneOf {
				fmt.Fprintf(b, "    case \"%s\":\n", opt)
			}
			fmt.Fprintf(b, "    default:\n        return fmt.Errorf(\"%s must be one of [%s]\")\n    }\n", v.YAMLPath, strings.Join(v.OneOf, " "))
		}
	}
	fmt.Fprintf(b, "    return nil\n}\n")
}

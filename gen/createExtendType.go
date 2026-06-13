package gen

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the schema of the provided config.json file

// GenerateDynaGenFile reads the config from configPath and flushes a formatted Go file to SQLPath + extType.go
func GenerateDynaGenFile(cfg *Config, extFileName string) error {
	// 3. Construct the literal string format for dynaOpt matching its structural type definition
	var dynaTableLines []string
	for k, v := range cfg.DynaTable {
		// Formats string arrays: []string{"a", "b"}
		var elements []string
		for _, s := range v {
			elements = append(elements, fmt.Sprintf("%q", s))
		}
		dynaTableLines = append(
			dynaTableLines,
			fmt.Sprintf("%q: {%s}", k, strings.Join(elements, ", ")),
		)
	}

	var refTableLines []string
	for k, v := range cfg.RefTable {
		refTableLines = append(refTableLines, fmt.Sprintf("%q: %q", k, v))
	}

	// Build the initialization literal chunk dynamically
	dynaOptValue := fmt.Sprintf(`DynaTable: map[string][]string{
		%s,
	},
	RefTable: map[string]string{
		%s,
	},
	RefTableName: %q,`,
		strings.Join(dynaTableLines, ",\n\t\t"),
		strings.Join(refTableLines, ",\n\t\t"),
		cfg.RefTableName,
	)

	// 4. Interpolate configuration data into the raw template literal

	rawGoCode := getExtDynaTypeGoLang(dynaOptValue, cfg.GetDynaQueryTable, cfg.GetDynaQueryFn)

	// 5. Run standard Go formatting (gofmt) on the generated string to fix spacing/indentations

	formattedCode, err := format.Source([]byte(rawGoCode))
	if err != nil {
		return fmt.Errorf("failed to format generated code (template syntax error): %w", err)
	}

	// 6. Define destination filepath path: sqlPath + extType.go
	// Ensure directory structure exists first
	if err := os.MkdirAll(cfg.SQLPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	outputPath := filepath.Join(cfg.SQLPath, extFileName)

	// 7. Flush formatted file content to disk
	if err := os.WriteFile(outputPath, formattedCode, 0o644); err != nil {
		return fmt.Errorf("failed to write generated Go file: %w", err)
	}

	fmt.Printf("flushed sqlc dyna file to: %s\n", outputPath)
	return nil
}

package gen

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/**
package sqlc

type tTbAllow map[string]bool

var (
  allowTableGuard = []string{ "cashflow", "to_cashflow" }
	cashflowGuard = tTbAllow{
		"income":     true,
		"expense":    true,
		"receivable": true,
		"payable":    true,
	}
	to_cashflowGuard = tTbAllow{
		"income":     true,
		"expense":    true,
		"receivable": true,
		"payable":    true,
	}
)
**/

func CreateExtendType(config *Config) error {
	Guard := config.Name.Guard
	tTbGuard := config.Name.TbGuard
	dynaTbGuard := config.Name.DynaTbGuard

	var src strings.Builder

	src.WriteString("package sqlc\n\n")
	fmt.Fprintf(&src, "type %s map[string]bool\n", tTbGuard)

	//'cashflow','to_cash','ref_cash','cash_inex'
	var keys []string
	var keysString []string
	for k := range config.DynaTable {
		keys = append(keys, k)
		keysString = append(keysString, fmt.Sprintf(`"%s"`, k))
	}

	sort.Strings(keys)
	sort.Strings(keysString)

	// start of var groupd
	src.WriteString("var (\n")

	// dynaTableGuard   = []string{"to_cash", "cashflow" }
	fmt.Fprintf(&src, "\t%s = []string{%s}\n", dynaTbGuard, strings.Join(keysString, ","))

	// each 'cashflow','to_cash','ref_cash','cash_inex'
	for _, key := range keys {
		//'income','expense'
		values := config.DynaTable[key]

		// Build elements: "income":true, "expense":true ...
		var elements []string
		for _, v := range values {
			elements = append(elements, fmt.Sprintf("%q: true", v))
		}

		// Join array elements together
		mapContent := strings.Join(elements, ", ")

		// cash_creditGuard = tTbAllow{"receivable": true, "payable": true}
		fmt.Fprintf(&src, "\t%s%s = %s{%s}\n", key, Guard, tTbGuard, mapContent)
	}

	// end of var group
	src.WriteString(")\n")

	formattedCode, err := format.Source([]byte(src.String()))
	if err != nil {
		return fmt.Errorf("failed to automatically format generated code: %w", err)
	}
	outputPath := filepath.Join(config.SQLPath, config.Name.FileTypeName)

	// 6. Ensure directory path existence before flushing data
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// 7. Flush raw formatted outputs to disk safely
	err = os.WriteFile(outputPath, formattedCode, 0o644)
	if err != nil {
		return fmt.Errorf("failed writing target file output: %w", err)
	}

	fmt.Printf("Successfully generated and formatted: %s\n", outputPath)
	return nil
}

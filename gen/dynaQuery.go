package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"regexp"
	"sort"
	"strings"
)

// GenerateDynamicQueryBuilders processes a raw SQL statement alongside target dynamic tables
// to output fully formatted Go code including parameters and validation logic.
func GenerateDynamicQueryBuilders(
	queryName string,
	sqlQuery string,
	allowTableGuard []string,
) (string, error) {
	// 1. Identify which tables from allowTableGuard appear in the SQL text.
	// Sort guards by length descending to prevent substring matching bugs (e.g., 'cashflow' inside 'to_cashflow').
	sortedGuards := make([]string, len(allowTableGuard))
	copy(sortedGuards, allowTableGuard)
	sort.Slice(sortedGuards, func(i, j int) bool {
		return len(sortedGuards[i]) > len(sortedGuards[j])
	})

	var foundGuards []string
	// Word boundaries or strict containment checks for SQL identifiers
	for _, guard := range sortedGuards {
		// Matching table identifier rules cleanly
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(guard) + `\b`)
		if re.MatchString(sqlQuery) {
			foundGuards = append(foundGuards, guard)
		}
	}

	// Re-sort matched guards alphabetically to ensure stable struct field ordering
	sort.Strings(foundGuards)

	if len(foundGuards) == 0 {
		return "", fmt.Errorf("no matching guards found inside the provided SQL query")
	}

	// 2. Format names for structs and helper functions
	// Convert snake_case or standard words to Capitalized camelCase identifiers for Struct fields.
	toCamelCase := func(s string) string {
		parts := strings.Split(s, "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
		return strings.Join(parts, "")
	}

	structName := fmt.Sprintf("Allow%sParams", toCamelCase(queryName))
	builderFuncName := fmt.Sprintf("get%sQuery", toCamelCase(queryName))

	// 3. Assemble Struct definition dynamically
	var src bytes.Buffer
	src.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	for _, guard := range foundGuards {
		src.WriteString(fmt.Sprintf("\t%s string\n", toCamelCase(guard)))
	}
	src.WriteString("}\n\n")

	// 4. Assemble Validation Conditions & Error messaging
	var validationChecks []string
	var errorFields []string

	for _, guard := range foundGuards {
		fieldName := toCamelCase(guard)
		validationChecks = append(
			validationChecks,
			fmt.Sprintf("!%sGuard[tb.%s]", guard, fieldName),
		)
		errorFields = append(errorFields, fmt.Sprintf("tb.%s", fieldName))
	}

	errorStr := "[]string{" + strings.Join(errorFields, ",") + "}"

	// 5. Construct the string split interpolation logic for the SQL template
	// Replace raw SQL keywords with dynamic concatenations safely
	replacements := make([]string, 0, len(foundGuards)*2)
	for _, guard := range foundGuards {
		replacements = append(replacements, guard, "` + tb."+toCamelCase(guard)+" + `")
	}
	replacer := strings.NewReplacer(replacements...)
	interpolatedSQL := replacer.Replace(sqlQuery)

	// Wrap code blocks into an optimized backtick string output
	src.WriteString(fmt.Sprintf("func %s(tb %s) (string, error) {\n", builderFuncName, structName))
	src.WriteString(fmt.Sprintf("\tif %s {\n", strings.Join(validationChecks, " || \n ")))
	src.WriteString(
		fmt.Sprintf(
			"\t\treturn \"\", errors.New(strings.Join(%s, \",\") + \"! not allowed.\")\n",
			errorStr,
		),
	)
	src.WriteString("\t}\n\n")
	src.WriteString(fmt.Sprintf("\treturn `%s`, nil\n", interpolatedSQL))
	src.WriteString("}\n")

	// 6. Run the whole payload back through standard go/format toolchain
	formattedCode, err := format.Source(src.Bytes())
	if err != nil {
		return "", fmt.Errorf(
			"failed to format generated code: %w\nRaw Source:\n%s",
			err,
			src.String(),
		)
	}

	return string(formattedCode), nil
}

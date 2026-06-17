package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"regexp"
	"strings"
)

// type AddCashflowDynaTbGuard struct {
func guardTbStructName(conTbName string, guardName string) string {
	return fmt.Sprintf("%sTb%s", toStructName(conTbName), guardName)
}

// func getAddCashflowDynaQuery(tb AllowAddCashflowDynaParams ) (string, error) {
func getTbQueryFnName(conTbName string) string {
	return fmt.Sprintf("get%sQuery", toStructName(conTbName))
}

type miWriter struct {
	src         bytes.Buffer
	structName  string
	fnQueryName string
	matchTable  map[string]string
	dynaQuery   string
	config      *Config
}

func GenerateTbStructTypeAndFnGetDynaQuery(
	constName string,
	constValueAsSql string,
	config *Config,
) (string, error) {
	matchTable, dynaQuery := GetDynaQueryInsensitive(constValueAsSql, config.AllowTableMap)
	w := miWriter{
		src:         bytes.Buffer{},
		structName:  guardTbStructName(constName, config.Name.Guard),
		fnQueryName: getTbQueryFnName(constName),
		matchTable:  matchTable,
		dynaQuery:   dynaQuery,
		config:      config,
	}

	GenerateStructType(&w)
	// fmt.Println(w.src.String())
	GenerateFnGetDynaQuery(&w)

	formattedCode, err := format.Source(w.src.Bytes())
	if err != nil {
		return "", fmt.Errorf(
			"failed to format generated code: %w\nRaw Source:\n%s",
			err,
			w.src.String(),
		)
	}

	return string(formattedCode), nil
}

func GenerateStructType(w *miWriter) {
	// build struct type like this

	// type AddCashflowTbGuard struct {
	// 	Cashflow string
	// 	ToCashflow string
	// }

	fmt.Fprintf(&w.src, "\ntype %s struct {\n", w.structName)
	for _, tbStruct := range w.matchTable {
		fmt.Fprintf(&w.src, "\t%s string\n", tbStruct)
	}
	w.src.WriteString("}\n\n")
}

func GenerateFnGetDynaQuery(w *miWriter) {
	/**
	build this fn
	func getAddCashflowQuery(tb AddCashflowTbGuard) (string, error) {
		if !to_cashflowGuard[tb.ToCashflow] ||
			!cashflowGuard[tb.Cashflow] {
			return "", errors.New(tb.ToCashflow + ", " + tb.Cashflow + "!not allow.")
		}

		return `
		insert into ` + tb.Cashflow + `(
				id,name
		)select id,name from ` + tb.ToCashflow + ` as c
		where c.id = $1
		returning id as newID
	`, nil
	}
	**/

	var validationChecks []string
	var errorFields []string

	for tb, tbStruct := range w.matchTable {
		validationChecks = append(
			validationChecks,
			//!cashflowGuard[tb.Cashflow]
			fmt.Sprintf("!%s%s[tb.%s]", tb, w.config.Name.Guard, tbStruct),
		)
		errorFields = append(errorFields, fmt.Sprintf("tb.%s", tbStruct))
	}

	validationStr := strings.Join(validationChecks, " || \n ")
	errorStr := strings.Join(errorFields, ` + ", " + `) + ` + " !not allow."`

	// Wrap code blocks into an optimized backtick string output
	fmt.Fprintf(&w.src, "func %s(tb %s) (string, error) {\n", w.fnQueryName, w.structName)
	fmt.Fprintf(&w.src, "\tif %s {\n", validationStr)
	fmt.Fprintf(
		&w.src,
		"\t\treturn \"\", errors.New(%s)\n", errorStr,
	)
	w.src.WriteString("\t}\n\n")
	fmt.Fprintf(&w.src, "\treturn `%s`, nil\n", w.dynaQuery)
	w.src.WriteString("}\n\n")
}

func GetDynaQueryInsensitive(
	sqlValue string,
	allowTableMap map[string]string,
) (matchTable map[string]string, dynaQuery string) {
	//{'cashflow':'Cashflow','to_cashflow':'ToCashflow'}
	matchTable = map[string]string{}
	var finalLines []string

	// Regex to extract whole words / alphanumeric identifiers containing underscores

	// Scan through the query line by line
	scanner := bufio.NewScanner(strings.NewReader(sqlValue))
	ln := 0
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Rule 1: If line starts with SQL comment "--", skip parsing this line and keep it as is
		if strings.HasPrefix(trimmed, "--") {
			// if ln == 0 {
			// 	finalLines = append(finalLines, "\n")
			// }
			continue
		}

		// Separate actual SQL code from any trailing inline comments (e.g., "id,name --,total")
		codePart, _, _ := strings.Cut(line, "--")

		// Track matches found strictly within this line's non-commented code
		// Using a reverse tracking map or sorting replacements helps if one table name is a substring of another.
		// However, regex matching on complete words handles boundary tracking natively.

		wordRegex := regexp.MustCompile(`[a-zA-Z0-9_]+`)
		updatedCodePart := wordRegex.ReplaceAllStringFunc(codePart, func(word string) string {
			lowerWord := strings.ToLower(word)
			// Rule 2: Check case-insensitively if word matches an allowed key
			if structName, has := allowTableMap[lowerWord]; has {
				// structName := toStructName(lowerWord)

				matchTable[lowerWord] = structName

				// Rule 2.2: Replace matched word with "tb.${structName}"
				return fmt.Sprintf("` + tb.%s + `", structName)
			}
			return word
		})

		// Reassemble the line with its original trailing comment intact
		finalLines = append(finalLines, updatedCodePart)
		ln++
	}

	dynaQuery = "\n" + strings.Join(finalLines, "\n") + "\n"
	return matchTable, dynaQuery
}

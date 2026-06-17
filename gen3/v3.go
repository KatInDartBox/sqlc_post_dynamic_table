package gen3

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"unicode"
)

// TableMatch maps a SQL table name to its Go struct field name
type TableMatch struct {
	Value      string // e.g. "cashflow"
	StructName string // e.g. "Cashflow"
}

// ---- helpers ----------------------------------------------------------------

// toTitle uppercases the first letter
func toTitle(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// constVarName converts "AddCashflow" → "addCashflow"
func toLower1(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// zeroValue returns the zero literal for a Go type expression string
func zeroValue(typeStr string) string {
	switch typeStr {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune":
		return "0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "error":
		return "nil"
	default:
		if strings.HasPrefix(typeStr, "*") || strings.HasPrefix(typeStr, "[]") {
			return "nil"
		}
		return typeStr + "{}"
	}
}

// exprToString converts an ast.Expr to its string representation
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// findTablesInSQL searches the SQL string for table names from tableMatches (case-insensitive).
// It uses word-boundary matching so "cashflow" doesn't match "ref_cashflow".
func findTablesInSQL(sql string, tableMatches []TableMatch) []TableMatch {
	found := []TableMatch{}
	sqlLower := strings.ToLower(sql)
	for _, tm := range tableMatches {
		pattern := `(?i)\b` + regexp.QuoteMeta(tm.Value) + `\b`
		re := regexp.MustCompile(pattern)
		if re.MatchString(sqlLower) {
			found = append(found, tm)
		}
	}
	return found
}

// replaceTableNamesInSQL replaces hard-coded table names with Go string concat:
//
//	"... from cashflow ..." → `... from ` + arg.Cashflow + ` ...`
func replaceTableNamesInSQL(sql string, foundTables []TableMatch) string {
	// Strip the leading comment line "-- name: XYZ :one\n"
	commentRe := regexp.MustCompile(`(?m)^--.*\n?`)
	sql = commentRe.ReplaceAllString(sql, "")
	sql = strings.TrimSpace(sql)

	for _, tm := range foundTables {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(tm.Value) + `\b`)
		sql = re.ReplaceAllString(sql, "` + arg."+tm.StructName+" + `")
	}
	return sql
}

// ---- AST analysis -----------------------------------------------------------

type funcInfo struct {
	name        string   // "AddCashflow"
	constName   string   // "addCashflow"  (the constant whose value is the query)
	sqlValue    string   // raw SQL string from the constant
	params      []string // Go type strings of params (after ctx), e.g. ["int64"] or ["PaidDeleteParams"]
	returns     []string // Go type strings, e.g. ["int64","error"]
	existStruct string   // if second param is already a struct type, its name; else ""
	node        *ast.FuncDecl
}

// findConstValue looks for `const <name> = \`...\“ in the file and returns its value
func findConstValue(file *ast.File, constName string) string {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name.Name == constName {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
							// Strip backticks
							val := lit.Value
							val = strings.TrimPrefix(val, "`")
							val = strings.TrimSuffix(val, "`")
							return val
						}
					}
				}
			}
		}
	}
	return ""
}

// collectFuncInfos parses the file and extracts info about each method on *Queries
func collectFuncInfos(file *ast.File) []funcInfo {
	var infos []funcInfo

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
			continue
		}

		// Must be a method on *Queries
		recvType := exprToString(fd.Recv.List[0].Type)
		if recvType != "*Queries" {
			continue
		}

		info := funcInfo{
			name: fd.Name.Name,
			node: fd,
		}

		// Collect params (skip ctx)
		if fd.Type.Params != nil {
			for i, field := range fd.Type.Params.List {
				if i == 0 {
					continue // skip ctx
				}
				typeStr := exprToString(field.Type)
				info.params = append(info.params, typeStr)
			}
		}

		// Collect returns
		if fd.Type.Results != nil {
			for _, field := range fd.Type.Results.List {
				info.returns = append(info.returns, exprToString(field.Type))
			}
		}

		// Find the constant name used in the function body (first Ident used as query arg)
		// Pattern: q.db.QueryRowContext(ctx, <constName>, ...) or q.db.ExecContext(ctx, <constName>, ...)
		info.constName = findQueryConstInBody(fd.Body)

		infos = append(infos, info)
	}
	return infos
}

// findQueryConstInBody walks the body to find the constant name passed as the query string
// to QueryRowContext / ExecContext / QueryContext
func findQueryConstInBody(body *ast.BlockStmt) string {
	result := ""
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		methodName := sel.Sel.Name
		if methodName != "QueryRowContext" && methodName != "ExecContext" &&
			methodName != "QueryContext" {
			return true
		}
		// Second argument (index 1) is the query constant
		if len(call.Args) >= 2 {
			if ident, ok := call.Args[1].(*ast.Ident); ok {
				result = ident.Name
			}
		}
		return false
	})
	return result
}

// isPrimitiveOrAbsent returns true if the type is a primitive / not a named struct
func isPrimitiveType(typeStr string) bool {
	primitives := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "string": true, "bool": true,
		"byte": true, "rune": true,
	}
	return primitives[typeStr] || strings.HasPrefix(typeStr, "*") ||
		strings.HasPrefix(typeStr, "[]")
}

// ---- Code generation --------------------------------------------------------

// generateGetQueryFunc builds the `getXxxQuery` function source
func generateGetQueryFunc(fnName, structName, dynamicSQL string, foundTables []TableMatch) string {
	getterName := "get" + toTitle(fnName) + "Query"

	// Build validation condition
	var conditions []string
	for _, tm := range foundTables {
		conditions = append(conditions, "!validateTb[arg."+tm.StructName+"]")
	}
	condition := strings.Join(conditions, " || ")

	// Build error message concatenation
	var errParts []string
	for i, tm := range foundTables {
		if i == 0 {
			errParts = append(errParts, "arg."+tm.StructName)
		} else {
			errParts = append(errParts, `" , "`+" + arg."+tm.StructName)
		}
	}
	errParts = append(errParts, `"! not found"`)
	errMsg := strings.Join(errParts, " + ")

	var sb strings.Builder
	sb.WriteString("func " + getterName + "(arg " + structName + ") (string, error) {\n")
	sb.WriteString("\tif " + condition + " {\n")
	sb.WriteString("\t\treturn \"\", errors.New(" + errMsg + ")\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\treturn `\n")
	sb.WriteString(dynamicSQL)
	sb.WriteString("\n\t`, nil\n")
	sb.WriteString("}\n")
	return sb.String()
}

// generateParamsStruct generates a new struct for functions that don't have one
func generateParamsStruct(
	structName string,
	existingFields string,
	foundTables []TableMatch,
) string {
	var sb strings.Builder
	sb.WriteString("type " + structName + " struct {\n")
	if existingFields != "" {
		sb.WriteString(existingFields)
	}
	for _, tm := range foundTables {
		sb.WriteString("\t" + tm.StructName + " string\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// ---- Struct field injection -------------------------------------------------

// findStructDecl finds a GenDecl containing a struct type with the given name
func findStructDecl(file *ast.File, structName string) *ast.GenDecl {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == structName {
				return gd
			}
		}
	}
	return nil
}

// ---- Source-level text transformation --------------------------------------

// handleSql is the main entry point: reads the file, applies transformations, writes back.
func HandleSql(filePath string, tableMatches []TableMatch) error {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	// Collect all function infos
	funcInfos := collectFuncInfos(file)

	// We'll collect all the const names we need to delete
	constsToDelete := map[string]bool{}

	// For each function, compute what we need
	type transformation struct {
		info          funcInfo
		foundTables   []TableMatch
		structName    string // params struct name
		isNewStruct   bool   // whether we need to create a new struct
		dynamicSQL    string
		getterFuncSrc string
		newStructSrc  string
	}

	var transforms []transformation

	for _, info := range funcInfos {
		if info.constName == "" {
			continue
		}

		sqlVal := findConstValue(file, info.constName)
		if sqlVal == "" {
			continue
		}

		foundTables := findTablesInSQL(sqlVal, tableMatches)
		if len(foundTables) == 0 {
			continue
		}

		// Determine struct name
		structName := ""
		isNewStruct := false

		if len(info.params) == 1 && !isPrimitiveType(info.params[0]) {
			// Already has a struct param – use it
			structName = info.params[0]
		} else {
			// Need to create a new struct
			structName = info.name + "Params"
			isNewStruct = true
		}

		dynamicSQL := replaceTableNamesInSQL(sqlVal, foundTables)

		getterSrc := generateGetQueryFunc(info.name, structName, dynamicSQL, foundTables)

		var newStructSrc string
		if isNewStruct {
			// Build existing primitive fields
			var existing strings.Builder
			if fd := info.node; fd.Type.Params != nil {
				for i, field := range fd.Type.Params.List {
					if i == 0 {
						continue // skip ctx
					}
					typeStr := exprToString(field.Type)
					if isPrimitiveType(typeStr) {
						for _, name := range field.Names {
							existing.WriteString("\t" + toTitle(name.Name) + " " + typeStr + "\n")
						}
					}
				}
			}
			newStructSrc = generateParamsStruct(structName, existing.String(), foundTables)
		} else {
			// Inject fields into existing struct
			newStructSrc = "" // handled separately via text replacement
		}

		constsToDelete[info.constName] = true

		transforms = append(transforms, transformation{
			info:          info,
			foundTables:   foundTables,
			structName:    structName,
			isNewStruct:   isNewStruct,
			dynamicSQL:    dynamicSQL,
			getterFuncSrc: getterSrc,
			newStructSrc:  newStructSrc,
		})
	}

	// -------------------------------------------------------------------------
	// Now we rebuild the file as text because AST rewriting of complex bodies
	// is extremely verbose. We use targeted regex/string replacements on source.
	// -------------------------------------------------------------------------

	output := string(src)

	// 1. Add "errors" import if not present
	if !strings.Contains(output, `"errors"`) {
		output = strings.Replace(output, `"context"`, `"context"`+"\n\t\"errors\"", 1)
	}

	// 2. Delete all const blocks we no longer need
	for constName := range constsToDelete {
		// Match: const <name> = `...`  (multiline backtick string)
		// Also handles the pattern where it's a standalone const declaration
		reConst := regexp.MustCompile(
			`(?s)const ` + regexp.QuoteMeta(constName) + ` = ` + "`" + `.*?` + "`" + `\s*\n?`,
		)
		output = reConst.ReplaceAllString(output, "")
	}

	// 3. For each transform, apply changes
	for _, t := range transforms {
		fnName := t.info.name
		structName := t.structName

		// 3a. If existing struct, inject new string fields
		if !t.isNewStruct {
			// Find the closing brace of the struct and insert before it
			structPattern := regexp.MustCompile(
				`(?s)(type ` + regexp.QuoteMeta(structName) + ` struct \{)(.*?)(\})`,
			)
			output = structPattern.ReplaceAllStringFunc(output, func(match string) string {
				sub := structPattern.FindStringSubmatch(match)
				if sub == nil {
					return match
				}
				open, body, close_ := sub[1], sub[2], sub[3]
				// Check which fields already exist
				for _, tm := range t.foundTables {
					if !strings.Contains(body, tm.StructName+" string") {
						body += "\t" + tm.StructName + " string\n"
					}
				}
				return open + body + close_
			})
		} else {
			// 3b. New struct: insert before the function declaration
			// Find the function declaration
			fnDeclPattern := regexp.MustCompile(`(?m)^func \(q \*Queries\) ` + regexp.QuoteMeta(fnName) + `\(`)
			loc := fnDeclPattern.FindStringIndex(output)
			if loc != nil {
				insertPos := loc[0]
				output = output[:insertPos] + t.newStructSrc + "\n" + output[insertPos:]
			}
		}

		// 3c. Insert getter function before the method
		fnDeclPattern := regexp.MustCompile(
			`(?m)^func \(q \*Queries\) ` + regexp.QuoteMeta(fnName) + `\(`,
		)
		loc := fnDeclPattern.FindStringIndex(output)
		if loc != nil {
			insertPos := loc[0]
			output = output[:insertPos] + t.getterFuncSrc + "\n" + output[insertPos:]
		}

		// 3d. Modify the function signature: replace/add second param with `arg <StructName>`
		//     and update the body.
		output = transformFuncDecl(output, t.info, structName)
	}

	// 4. Clean up double blank lines
	doubleBlank := regexp.MustCompile(`\n{3,}`)
	output = doubleBlank.ReplaceAllString(output, "\n\n")

	// 5. Format with gofmt
	formatted, err := format.Source([]byte(output))
	if err != nil {
		// Write unformatted so user can debug
		fmt.Fprintf(os.Stderr, "gofmt error (writing unformatted): %v\n", err)
		return os.WriteFile(filePath, []byte(output), 0o644)
	}

	return os.WriteFile(filePath, formatted, 0o644)
}

// transformFuncDecl rewrites a single method's signature and body in the source text.
func transformFuncDecl(src string, info funcInfo, structName string) string {
	fnName := info.name

	// We need to:
	// (a) Change signature to use `arg <structName>` as second param
	// (b) Replace the body to use dynaQuery

	// ---- Signature rewrite --------------------------------------------------
	// Match: func (q *Queries) FnName(ctx context.Context, <oldParams>) <returns>
	sigRe := regexp.MustCompile(
		`(?m)(func \(q \*Queries\) ` + regexp.QuoteMeta(
			fnName,
		) + `\(ctx context\.Context)(,?[^)]*?)(\))`,
	)

	// Determine new signature suffix
	newSigSuffix := ", arg " + structName

	src = sigRe.ReplaceAllStringFunc(src, func(match string) string {
		sub := sigRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		prefix := sub[1] // "func (q *Queries) FnName(ctx context.Context"
		close_ := sub[3] // ")"
		return prefix + newSigSuffix + close_
	})

	// ---- Body rewrite -------------------------------------------------------
	// We replace the entire function body.
	// Strategy: find the function, extract return types, find the db call, reconstruct.

	// Find the function block in source
	funcStartRe := regexp.MustCompile(
		`(?s)(func \(q \*Queries\) ` + regexp.QuoteMeta(fnName) + `\([^)]*\)[^{]*)\{(.*?)\n\}`,
	)

	src = funcStartRe.ReplaceAllStringFunc(src, func(match string) string {
		sub := funcStartRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		sig := sub[1]
		body := sub[2]

		newBody := rewriteFuncBody(body, info, structName)
		return sig + "{\n" + newBody + "\n}"
	})

	return src
}

// rewriteFuncBody rewrites the body of a transformed function.
func rewriteFuncBody(body string, info funcInfo, structName string) string {
	fnName := info.name
	getterCall := "get" + toTitle(fnName) + "Query"

	// Determine zero returns (everything except the last "error")
	nonErrReturns := []string{}
	for _, r := range info.returns {
		if r != "error" {
			nonErrReturns = append(nonErrReturns, zeroValue(r))
		}
	}

	// Build the early-return line
	var earlyReturn string
	if len(nonErrReturns) > 0 {
		earlyReturn = "return " + strings.Join(append(nonErrReturns, "errQuery"), ", ")
	} else {
		earlyReturn = "return errQuery"
	}

	// Build preamble
	preamble := fmt.Sprintf(
		"\tdynaQuery, errQuery := %s(arg)\n\tif errQuery != nil {\n\t\t%s\n\t}\n",
		getterCall, earlyReturn,
	)

	// Replace the constant name with dynaQuery in the db call
	constName := info.constName
	newBody := strings.ReplaceAll(body, constName, "dynaQuery")

	// Remove the old `var id` line if we changed the parameter to arg
	// Also replace bare `id` references with `arg.<Field>` for primitive cases
	// (This is heuristic – for the common sqlc pattern it works well)
	newBody = strings.TrimSpace(newBody)

	return preamble + newBody
}

// ---- main -------------------------------------------------------------------

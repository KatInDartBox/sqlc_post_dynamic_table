package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type funcCallSite struct {
	fn       *ast.FuncDecl
	callExpr *ast.CallExpr // the QueryRowContext / ExecContext call
	constArg int           // argument index holding the constant (always 1)
	name     string
}

// dynaConst holds the analysed metadata for a matched constant.
type dynaConst struct {
	name                        string // constant identifier, e.g. "addCashflow"
	value                       string // raw SQL string value
	conDecl                     *ast.GenDecl
	structTypeAndFnGetDynaQuery string
	fn                          funcCallSite
}

func getAllConsts(file *ast.File, config *Config) []dynaConst {
	var consts []dynaConst

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				// strip surrounding backticks or quotes
				raw := lit.Value
				unquoted := raw[1 : len(raw)-1]
				dynaQuery, err := GenerateTbStructTypeAndFnGetDynaQuery(name.Name, unquoted, config)
				if err != nil || dynaQuery == "" {
					continue
				}

				consts = append(consts, dynaConst{
					name:                        name.Name,
					value:                       unquoted,
					conDecl:                     genDecl,
					structTypeAndFnGetDynaQuery: dynaQuery,
				})
			}
		}
	}

	return consts
}

func getFilterConstsBySqlExecFn(
	file *ast.File,
	allConsts *[]dynaConst,
) (filterConsts []dynaConst) {
	var matched []dynaConst
	matchedObj := map[string]dynaConst{}

	// filter constance that contain dynatable keys
	for _, c := range *allConsts {
		matchedObj[c.name] = c
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			// check if body has fn
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// second argument (index 1) should be the constant identifier
			// check if that fn as arg more that 2
			if len(call.Args) < 2 {
				return true
			}

			// index is query constance name like addCashflow
			ident, ok := call.Args[1].(*ast.Ident)
			if !ok {
				return true
			}

			used, usedFnArg := matchedObj[ident.Name]
			if usedFnArg {

				used.fn = funcCallSite{
					fn:       fn,
					callExpr: call,
					constArg: 1,
				}

				matched = append(matched, used)
				// matched = append(matched, dynaConst{
				// 	name:                        ident.Name,
				// 	value:                       used.value,
				// 	conDecl:                     used.conDecl,
				// 	structTypeAndFnGetDynaQuery: used.structTypeAndFnGetDynaQuery,
				// 	fn: funcCallSite{
				// 		fn:       fn,
				// 		callExpr: call,
				// 		constArg: 1,
				// 	},
				// })
			}

			return true
		})
	}

	return matched
}

func getZeroLiteral(expr ast.Expr, fset *token.FileSet) string {
	switch t := expr.(type) {
	// plain identifier: bool, string, int*, uint*, float*, error, or a named type
	case *ast.Ident:
		switch t.Name {
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64", "byte", "rune", "uintptr":
			return "0"
		case "bool":
			return "false"
		case "string":
			return `""`
		case "error":
			return `errQuery`
		default:
			// named type — assume struct, emit T{}
			return t.Name + "{}"
		}
	// qualified identifier: pkg.Type -> pkg.Type{}
	case *ast.SelectorExpr:
		var buf bytes.Buffer
		format.Node(&buf, fset, t)
		return buf.String() + "{}"
	// *T, []T, map[K]V, chan T -> nil
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType:
		return "nil"
	// interface{} / any -> nil
	case *ast.InterfaceType:
		return "nil"
	// anonymous inline struct -> struct{}{}
	case *ast.StructType:
		return "struct{}{}"
	default:
		return "nil"
	}
}

func zeroValueOf(fn *ast.FuncDecl, fset *token.FileSet) []string {
	if fn.Type.Results == nil {
		return nil
	}
	var zeros []string
	for _, field := range fn.Type.Results.List {
		count := 1
		if len(field.Names) > 0 {
			count = len(field.Names)
		}
		z := getZeroLiteral(field.Type, fset)
		for i := 0; i < count; i++ {
			zeros = append(zeros, z)
		}
	}
	return zeros
}

func getTbGuardInsideSqlExeFn(zerosReturn []string, constName string) string {
	guardQueryFn := getTbQueryFnName(constName)
	returnStmt := strings.Join(zerosReturn, ", ")
	temp := `
	dynaQuery, errQuery := %s(dynaTable)
	if errQuery != nil {
		return %s
	}
`
	guard := fmt.Sprintf(temp, guardQueryFn, returnStmt)

	return guard
}

func HandleSql(sqlPath string, config *Config) error {
	src, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("handleSql: read file : %w ", err)
	}

	fset := token.NewFileSet()

	// helper: convert a token.Pos to a byte offset
	offset := func(p token.Pos) int {
		return fset.Position(p).Offset
	}

	file, err := parser.ParseFile(fset, sqlPath, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("handleSql: parse: %w", err)
	}

	// ── 1. collect all string constants ──────────────────────────────────────
	allConsts := getAllConsts(file, config)

	// ── 2. match constants against dynaTableMap ───────────────────────────────
	matched := getFilterConstsBySqlExecFn(file, &allConsts)

	if len(matched) == 0 {
		return nil // nothing to do
	}

	// ── 4. determine return-type zero values for each matched function ─────────

	// ── 5. rewrite the source as a byte slice ─────────────────────────────────
	//
	// Strategy: work on the raw []byte, applying edits from back to front so
	// that earlier offsets remain valid.

	var edits []edit

	//  For each matched function:
	//       - inject dynaTable parameter
	//       - inject lookup guard at top of body
	//       - replace constant arg with tb

	for _, match := range matched {

		cs := match.fn
		fn := cs.fn

		// ── inject dynaTable string parameter ──────────────────────────────
		// Find the position just after "ctx context.Context" in the param list.
		// We insert ", dynaTable string" after the first parameter.
		params := fn.Type.Params.List
		if len(params) == 0 {
			continue
		}
		// The first param is ctx; we insert after its closing type position.
		firstParamEnd := offset(params[0].Type.End())
		edits = append(edits, edit{
			pos:  firstParamEnd,
			end:  firstParamEnd,
			text: ", dynaTable " + guardTbStructName(match.name, config.Name.Guard),
		})

		// func getAddCashflowDynaQuery(tb AllowAddCashflowDynaParams ) (string, error) {
		// guardQueryFn := getTbQueryFnName(match.name)

		// ── inject lookup guard at top of function body ────────────────────
		bodyOpen := offset(fn.Body.Lbrace) + 1 // just after '{'
		zeros := zeroValueOf(fn, fset)
		guard := getTbGuardInsideSqlExeFn(zeros, match.name)

		edits = append(edits, edit{
			pos:  bodyOpen,
			end:  bodyOpen,
			text: guard,
		})

		// ── replace constant identifier with tb in the call ────────────────
		constArg := cs.callExpr.Args[cs.constArg]
		edits = append(edits, edit{
			pos:  offset(constArg.Pos()),
			end:  offset(constArg.End()),
			text: "dynaQuery",
		})
	}

	for _, m := range matched {
		// append struct type and fn getDynaQuery
		end := offset(m.conDecl.End())
		injection := m.structTypeAndFnGetDynaQuery
		edits = append(edits, edit{
			pos:  end,
			end:  end,
			text: injection,
		})

		// delete used const
		edits = append(edits, edit{
			pos:  offset(m.conDecl.Pos()),
			end:  end,
			text: "",
		})
	}

	// ensure "errors" is in the import block
	edits = append(edits, ensureImport(fset, file, "errors")...)

	// ── 6. apply edits back-to-front ─────────────────────────────────────────
	// sort descending by pos
	for i := 0; i < len(edits); i++ {
		for j := i + 1; j < len(edits); j++ {
			if edits[j].pos > edits[i].pos {
				edits[i], edits[j] = edits[j], edits[i]
			}
		}
	}

	result := make([]byte, len(src))
	copy(result, src)
	for _, e := range edits {
		result = append(result[:e.pos], append([]byte(e.text), result[e.end:]...)...)
	}

	// ── 7. gofmt the output ───────────────────────────────────────────────────
	formatted, err := format.Source(result)
	if err != nil {
		// write unformatted so the caller can inspect it
		_ = os.WriteFile(sqlPath, result, 0o644)
		return fmt.Errorf("handleSql: gofmt: %w", err)
	}

	if err := os.WriteFile(sqlPath, formatted, 0o644); err != nil {
		return fmt.Errorf("handleSql: write file: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// edit represents a single byte-range replacement inside a source file.
type edit struct {
	pos  int    // start offset (inclusive)
	end  int    // end offset (exclusive)
	text string // replacement text
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ensureImport returns an edit that adds the given import path if it is not
// already present in the file.
//
// Three cases:
//  1. Already imported           -> no edit
//  2. Grouped import ( ... )     -> insert the new path inside the parens
//  3. Bare import "pkg"          -> replace the GenDecl with a grouped block
//  4. No imports at all          -> insert a new import block after the package clause
func ensureImport(fset *token.FileSet, file *ast.File, pkg string) []edit {
	quoted := `"` + pkg + `"`

	// check all existing imports
	for _, imp := range file.Imports {
		if imp.Path.Value == quoted {
			return nil // already present
		}
	}

	// find the import GenDecl(s) in the file
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}
		if genDecl.Lparen.IsValid() {
			// grouped: insert just before the closing paren
			rparenOff := fset.Position(genDecl.Rparen).Offset
			return []edit{{pos: rparenOff, end: rparenOff, text: "\t" + quoted + "\n"}}
		}
		// bare: import "pkg"  -- replace the entire GenDecl with a grouped block
		start := fset.Position(genDecl.Pos()).Offset
		end := fset.Position(genDecl.End()).Offset
		var paths []string
		for _, spec := range genDecl.Specs {
			is, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}
			paths = append(paths, "\t"+is.Path.Value)
		}
		paths = append(paths, "\t"+quoted)
		replacement := "import (\n" + strings.Join(paths, "\n") + "\n)"
		return []edit{{pos: start, end: end, text: replacement}}
	}

	// no import block at all -- insert after the package name
	pkgEnd := fset.Position(file.Name.End()).Offset
	return []edit{{pos: pkgEnd, end: pkgEnd, text: "\n\nimport " + quoted}}
}

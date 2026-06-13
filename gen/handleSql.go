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
	name     string   // constant identifier, e.g. "addCashflow"
	value    string   // raw SQL string value
	fnName   string   // function that uses this constant
	objKey   string   // matched table key, e.g. "cashflow"
	objValue []string // accepted values for that key
	fn       funcCallSite
}

type constInfo struct {
	name    string
	value   string
	declEnd token.Pos // position just after the closing backtick / quote
}

func getAllConsts(file *ast.File) []constInfo {
	var consts []constInfo

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
				consts = append(consts, constInfo{
					name:    name.Name,
					value:   unquoted,
					declEnd: genDecl.End(),
				})
			}
		}
	}
	return consts
}

func getFilterConsts(
	file *ast.File,
	allConsts *[]constInfo,
	dynaTableMap map[string][]string,
) (filterConsts []dynaConst, constToCall map[string]funcCallSite) {
	var matched []dynaConst
	matchedObj := map[string]dynaConst{}

	// filter constance that contain dynatable keys
	for _, c := range *allConsts {
		lower := strings.ToLower(c.value)
		for tableKey, tableVals := range dynaTableMap {
			if strings.Contains(lower, tableKey) {
				matchedObj[c.name] = dynaConst{
					name:     c.name,
					value:    c.value,
					objKey:   tableKey,
					objValue: tableVals,
				}
				break
			}
		}
	}

	// constToCall = map[string]funcCallSite{} // key = const name

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

			ident, ok := call.Args[1].(*ast.Ident)
			if !ok {
				return true
			}

			used, usedFnArg := matchedObj[ident.Name]
			if usedFnArg {
				matched = append(matched, dynaConst{
					name:  ident.Name,
					value: used.value,
					fn: funcCallSite{
						fn:       fn,
						callExpr: call,
						constArg: 1,
					},
				})
			}

			// for i := range matched {
			// 	if ident.Name == matched[i].name {
			// 		// (matched[i]).fnName = fn.Name.Name
			// 		fnMatched := funcCallSite{
			// 			fn:       fn,
			// 			callExpr: call,
			// 			constArg: 1,
			// 		}
			// 		constToCall[ident.Name] = fnMatched
			// 		matched[i].fn = fnMatched
			// 	}
			// }
			return true
		})
	}

	return matched, constToCall
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
			return `errors.New("Table " + dynaTable + "! not found.")`
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

func HandleSql(sqlPath string, config *Config) error {
	dynaTableMap := config.DynaTable

	src, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("handleSql: read file : %w ", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sqlPath, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("handleSql: parse: %w", err)
	}

	// ── 1. collect all string constants ──────────────────────────────────────
	allConsts := getAllConsts(file)

	// ── 2. match constants against dynaTableMap ───────────────────────────────
	matched, _ := getFilterConsts(file, &allConsts, dynaTableMap)

	if len(matched) == 0 {
		return nil // nothing to do
	}

	// ── 4. determine return-type zero values for each matched function ─────────

	// ── 5. rewrite the source as a byte slice ─────────────────────────────────
	//
	// Strategy: work on the raw []byte, applying edits from back to front so
	// that earlier offsets remain valid.

	var edits []edit

	// helper: convert a token.Pos to a byte offset
	offset := func(p token.Pos) int {
		return fset.Position(p).Offset
	}

	// 5a. For each matched function:
	//       - inject dynaTable parameter
	//       - inject lookup guard at top of body
	//       - replace constant arg with tb

	for _, match := range matched {
		// cs, ok := constToCall[match.name]
		// if !ok {
		// 	continue
		// }

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
			text: ", dynaTable string",
		})

		// ── inject lookup guard at top of function body ────────────────────
		bodyOpen := offset(fn.Body.Lbrace) + 1 // just after '{'
		zeros := zeroValueOf(fn, fset)
		returnStmt := "return " + strings.Join(zeros, ", ")
		guard := fmt.Sprintf("\n\ttb, found := dyna%s[dynaTable]\n\tif !found {\n\t\t%s\n\t}\n",
			capitalize(match.name), returnStmt)
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
			text: "tb",
		})
	}

	// 5b. For each matched constant block, insert var + init() after it.
	//     We need the end offset of the GenDecl that contains the constant.
	constDeclEnd := map[string]int{} // const name → byte offset of GenDecl end
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
			for _, name := range vs.Names {
				constDeclEnd[name.Name] = offset(genDecl.End())
			}
		}
	}

	for _, m := range matched {
		end, ok := constDeclEnd[m.name]
		if !ok {
			continue
		}
		capName := capitalize(m.name)
		injection := fmt.Sprintf(
			"\n\nvar dyna%s = map[string]string{}\n\nfunc init() {\n\tdyna%s = %s(%s)\n}\n",
			capName,
			capName,
			config.GetDynaQueryFn,
			m.name,
		)
		edits = append(edits, edit{
			pos:  end,
			end:  end,
			text: injection,
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

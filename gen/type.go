package gen

import "strings"

type ConfigName struct {
	FileTypeName string `json:"fileTypeName"`
	Guard        string `json:"guard"`
	DynaTable    string `json:"dynaTable"`
	DynaTbGuard  string // dynaTableGuard
	TbGuard      string // tTbGuard
}

type Config struct {
	SQLPath       string              `json:"sqlPath"`
	DynaTable     map[string][]string `json:"dynaTable"`
	Name          ConfigName          `json:"name"`
	AllowTable    []string            //'cashflow','to_cashflow'
	AllowTableMap map[string]string   //{'cashflow':'Cashflow','to_cashflow':'ToCashflow'}

	// RefTable           map[string]string `json:"refTable"`
	// RefTableName       string            `json:"refTableName"`
	// GetDynaQueryFn     string            `json:"getDynaQueryFn"`
	// GetDynaQueryTable  string            `json:"getDynaQueryTable"`
	// ExtendDynaType     string            `json:"extendDynaType"`
	// ExtendTypeFileName string            `json:"extendTypeFileName"`
}

func upperFistStr(str string) string {
	return strings.ToUpper(str[:1]) + str[1:]
}

// toStructName converts snake_case or lowercase strings into Go-friendly CamelCase names
// e.g., "to_cashflow" -> "ToCashflow", "cashflow" -> "Cashflow"
func toStructName(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = upperFistStr(part)
		}
	}
	return strings.Join(parts, "")
}

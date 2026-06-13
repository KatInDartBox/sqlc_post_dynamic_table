package gen

type Config struct {
	SQLPath           string              `json:"sqlPath"`
	DynaTable         map[string][]string `json:"dynaTable"`
	RefTable          map[string]string   `json:"refTable"`
	RefTableName      string              `json:"refTableName"`
	GetDynaQueryFn    string              `json:"getDynaQueryFn"`
	GetDynaQueryTable string              `json:"getDynaQueryTable"`
}

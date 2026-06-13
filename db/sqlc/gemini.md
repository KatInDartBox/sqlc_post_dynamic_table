provided that i have a config.json file like follow:

```json
{
  "sqlPath": "/home/ubuntu/projects/go/src/sqlc_post_plugin/db/sqlc",
  "dynaTable": {
    "cashflow": ["income", "expense", "receivable", "payable"],
    "cash_inex": ["income", "expense"],
    "cash_credit": ["receivable", "payable"]
  },
  "refTableName": "ref_table",
  "refTable": {
    "income": "receivable",
    "expense": "payable",
    "receivable": "income",
    "payable": "expense"
  }
}
```

create a golang function read this config.json then flush
the following string as golang to provided sqlPath + extType.go

```go
package dynaGen

import (
	"strings"
)

type DynaConfig struct {
	DynaTable    map[string][]string `json:"dynaTable"`
	RefTable     map[string]string   `json:"refTable"`
	RefTableName string              `json:"refTableName"`
}

var dynaOpt = DynaConfig{
${getThisFromConfigAndMatchWithDynaConfigType}
}

func getDynaTable(query string) (dynaTb string, has bool) {
	for dynaTb = range dynaOpt.DynaTable {
		if strings.Contains(query, dynaTb) {
			return dynaTb, true
		}
	}
	return "", false
}

func GetDynaQuery(query string) (qMap map[string]string) {
	// cash_inex
	qMap = map[string]string{}

	dynaTableName, has := getDynaTable(query)
	if !has {
		return
	}
	refTableName := dynaOpt.RefTableName
	//[]{"income","expense"}
	tables := dynaOpt.DynaTable[dynaTableName]
	for _, dynaTable := range tables {
		refTable := dynaOpt.RefTable[dynaTable]
		replacer := strings.NewReplacer(dynaTableName, dynaTable, refTableName, refTable)
		qMap[dynaTable] = replacer.Replace(query)

	}

	return qMap
}
```

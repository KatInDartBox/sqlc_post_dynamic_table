package sqlc

import "strings"

/**
config.json =
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
**/

type DynaConfig struct {
	DynaTable    map[string][]string `json:"dynaTable"`
	RefTable     map[string]string   `json:"refTable"`
	RefTableName string              `json:"refTableName"`
}

var dynaOpt = DynaConfig{
	DynaTable: map[string][]string{
		"cashflow":    {"income", "expense", "receivable", "payable"},
		"cash_inex":   {"income", "expense"},
		"cash_credit": {"receivable", "payable"},
	},
	RefTable: map[string]string{
		"income":     "receivable",
		"expense":    "payable",
		"receivable": "income",
		"payable":    "expense",
	},
	RefTableName: "ref_table",
}

func getDynamicQueryTable(query string) (dynaTb string, has bool) {
	for dynaTb = range dynaOpt.DynaTable {
		if strings.Contains(query, dynaTb) {
			return dynaTb, true
		}
	}
	return "", false
}

/**
const paidDelete = `-- name: PaidDelete :exec
WITH cte_paid_delete as (
  delete from cash_inex as cold
  WHERE
    cold.id = $2
  returning cold.total
)
update ref_table as cnew set
  total = total + cte_paid_delete.total,
  name = $1
from cte_paid_delete
where
  cnew.id = cte_paid_delete.credit_ref
`
=> map[string]string{
income: `-- name: PaidDelete :exec
	WITH cte_paid_delete as (
		delete from income as cold
		WHERE
			cold.id = $2
		returning cold.total
	)
	update receivable as cnew set
		total = total + cte_paid_delete.total,
		name = $1
	from cte_paid_delete
	where
		cnew.id = cte_paid_delete.credit_ref` ,
expense:`-- name: PaidDelete :exec
	WITH cte_paid_delete as (
		delete from expense as cold
		WHERE
			cold.id = $2
		returning cold.total
	)
	update payable as cnew set
		total = total + cte_paid_delete.total,
		name = $1
	from cte_paid_delete
	where
		cnew.id = cte_paid_delete.credit_ref
`
}
**/

func getDynamicQuery(query string) (qMap map[string]string) {
	// cash_inex
	qMap = map[string]string{}

	dynaTableName, has := getDynamicQueryTable(query)
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

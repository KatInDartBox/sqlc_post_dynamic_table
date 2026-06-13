# sqlc post dynamic table

generate a dynamic table along with sqlc static table.
this binary will scan \*.sql.go modify it to support dynamic table.
sql file that has dynaTable [cashflow,cash_inex,cash_credit]
will be replace it with array of its corresponsive value.
then from this value if it has refTableName [ref_table] will be replace with
object lookup value.

for example:

1. add a sqlc_dyna.json at root of dyna binary

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
  },

  //file where getDynamicQueryTable live
  "extendDynaType": "extDynamicType.go",

  //dynamice query table function
  //name inside extDynamicType.go
  "getDynaQueryFn": "getDynamicQuery",
  "getDynaQueryTable": "getDynamicQueryTable"
}
```

```go
const addCashflow = `
  insert into cash_inex(
      id,name
  )select id,name from ref_table as c
  where c.id = $1
  returning id
`

var dynaAddCashflow = map[string]string{}

func init() {
	dynaAddCashflow = getDynamicQuery(addCashflow)
}
```

getDynamicQuery function will return

```go
dynaAddCashflow = map[string]string{
"income":`
  insert into income(
      id,name
  )select id,name from receivable as c
  where c.id = $1
  returning id
  `,
"expense":`
  insert into expense(
      id,name
  )select id,name from payable as c
  where c.id = $1
  returning id
  `,
}
```

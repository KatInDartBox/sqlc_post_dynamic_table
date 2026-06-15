provided that we have a config.json like bellow

##config.json

```json
{
  "allowTable": {
    "cashflow": ["income", "expense", "receivable", "payable"],
    "to_cashflow": ["income", "expense", "receivable", "payable"]
  }
}
```

create a golang fn its will read every key,val of allowTable then split out
template like bellow

##gen/extType.go

```go
type tTbAllow map[string]bool
var cashflowGuard = tTbAllow{"income":true, "expense":true, "receivable":true, "payable":true}
var to_cashflowGuard = tTbAllow{"income":true, "expense":true, "receivable":true, "payable":true}
```

provided that we have

1.an extType.go

```go
//extType.go

package sqlc

type tTbAllow map[string]bool

var (
  allowTableGuard = []string{ "cashflow", "to_cashflow" }
	cashflowGuard = tTbAllow{
		"income":     true,
		"expense":    true,
		"receivable": true,
		"payable":    true,
	}
	to_cashflowGuard = tTbAllow{
		"income":     true,
		"expense":    true,
		"receivable": true,
		"payable":    true,
	}
)

```

2.cashflow.sql.go

```go
package sqlc

import (
	"errors"
	"context"
)

const addCashflowDyna = `
  insert into cashflow(
      id,name
  )select id,name from to_cashflow as c
  where c.id = $1
  returning id
`
```

my goal is to create a golang fn which read query addCashflowDyna value as argument
a.check if its value contain oneof allowTableGuard string.
b.for every string it has in allowTableGuard create a table struct like so

```go
type AllowAddCashflowDynaParams struct {
	Cashflow string
	To_cashflow string
}
```

c.create a golang func output like bellow.

```go
func getAddCashflowDynaQuery(tb AllowAddCashflowDynaParams ) (string, error) {
	if !cashflowGuard[tb.Cashflow] ||
     !to_cashflowGuard[tb.To_cashflow] {
		return "", errors.New(tb.Cashflow + ", "+ tb.To_cashflow + "! not allowed.")
	}

	return `
  insert into ` + tb.Cashflow + `(
      id,name
  )select id,name from ` + tb.To_cashflow  + ` as c
  where c.id = $1
  returning id
`, nil
}
```

this fn will replace every found allowTableGuard with tb key. also check
if it is exists in respective guard.

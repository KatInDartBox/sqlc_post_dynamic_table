package sqlc

import (
	"context"
	"errors"
)

const addCashflowDyna = `
  insert into cashflow(
      id,name
  )select id,name from to_cashflow as c
  where c.id = $1
  returning id
`

type tTbAllow map[string]bool

var (
	allowTableGuard = []string{"cashflow", "to_cashflow"}

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

type AllowDynaTable struct {
	Cashflow    string
	To_cashflow string
}

/*
tb:{
cashflow:'income',
to_cashflow:'receivable',
}
*/
func getAddCashflowQuery(tb AllowDynaTable) (string, error) {
	cashflowOK := cashflowGuard[tb.Cashflow]
	to_cashflowOK := to_cashflowGuard[tb.To_cashflow]
	if !cashflowOK || !to_cashflowOK {
		return "", errors.New(tb.Cashflow + ", " + tb.To_cashflow + "! not allowed.")
	}

	return `
  insert into ` + tb.Cashflow + `(
      id,name
  )select id,name from ` + tb.To_cashflow + ` as c
  where c.id = $1
  returning id
`, nil
}

func (q *Queries) AddCashflowDyna(ctx context.Context, tb AllowDynaTable, id int64) (int64, error) {
	dynaQuery, errQ := getAddCashflowQuery(tb)
	if errQ != nil {
		return 0, errQ
	}

	row := q.db.QueryRowContext(ctx, dynaQuery, id)
	err := row.Scan(&id)
	return id, err
}

provided that i have a golang file cashflow.sql.go like bellow.

```go
package sqlc

import (
	"context"
)

const addCashflow = `-- name: AddCashflow :one
  insert into cashflow(
      id,name
  )select id,name from to_cashflow as c
  where c.id = $1
  returning id
`

func (q *Queries) AddCashflow(ctx context.context, id int64) (int64, error) {
	row := q.db.QueryRowContext(ctx, addCashflow, id)
	err := row.Scan(&id)
	return id, err
}

const paidDelete = `-- name: PaidDelete :exec
WITH cte_paid_delete as (
  delete from cash_inex as cold
  WHERE
    cold.id = $2
  returning cold.total
)
update ref_cash as cnew set
  total = total + cte_paid_delete.total,
  name = $1
from cte_paid_delete
where
  cnew.id = cte_paid_delete.credit_ref
`

type PaidDeleteParams struct {
	Name      string `json:"name"`
	InExDocID int64  `json:"in_ex_doc_id"`
}

func (q *Queries) PaidDelete(ctx context.Context, arg PaidDeleteParams) error {
	_, err := q.db.ExecContext(ctx, paidDelete, arg.Name, arg.InExDocID)
	return err
}

const flushOldCashflow = `-- name: FlushOldCashflow :exec
Delete from cashflow
WHERE
updated_at < current_date - 7
`

func (q *Queries) FlushOldCashflow(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, flushOldCashflow)
	return err
}
```

in golang, create a func call deleteConsts. this fn take arg of constNames []string{}.
this fn will loop through cashflow.sql.go and
delete any constance (its name, and value ) if it is existed in constNames;

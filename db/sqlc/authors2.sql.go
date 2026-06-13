package sqlc

import (
	"context"
)

const addCashflow2 = `-- name: AddCashflow :one
  insert into cashflow(
      id,name
  )select id,name from cashflow as c
  where c.id = $1
  returning id
`

func (q *Queries) AddCashflow2(ctx context.Context, id int64) (int64, error) {
	row := q.db.QueryRowContext(ctx, addCashflow, id)
	err := row.Scan(&id)
	return id, err
}

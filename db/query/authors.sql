-- name: AddCashflow :one
  insert into cashflow(
      id,name
  )select id,name from cashflow as c
  where c.id = $1
  returning id;

-- name: PaidDelete :exec
WITH cte_paid_delete as (
  delete from cash_inex as cold
  WHERE 
    cold.id = @in_ex_doc_id
  returning cold.total
)
update ref_table as cnew set
  total = total + cte_paid_delete.total,
  name = @name
from cte_paid_delete 
where 
  cnew.id = cte_paid_delete.credit_ref;


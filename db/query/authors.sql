-- name: AddCashflow :one
  insert into cashflow(
      id,name --,total
  )select id,name from to_cashflow as c
  where c.id = $1
  returning id as newID;

-- name: FlushCashflow :exec
  delete from  cashflow
  where c.id = 123 -- and total = 0
;


-- name: AuthDeleteIncome :exec
delete from income where 
id = @id and name = @name;

-- name: PaidDelete :exec
WITH cte_paid_delete as (
  -- name=9
  delete from cash_inex as cold
  WHERE 
    cold.id = @in_ex_doc_id
  returning cold.total
)
update ref_cash as cnew set
  total = total + cte_paid_delete.total,
  name = @name
from cte_paid_delete 
where 
  cnew.id = cte_paid_delete.credit_ref;


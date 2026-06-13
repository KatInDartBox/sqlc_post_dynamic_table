-- name: GetCashIncome :many
select * from income
where id = @id and name = @name;


-- name: DeleteCashIncome :exec
delete from income where 
id = @id and name = @name;


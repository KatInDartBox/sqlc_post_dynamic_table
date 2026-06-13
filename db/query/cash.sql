-- name: GetCashflow :many
select * from cashflow 
where id = @id and name = @name;


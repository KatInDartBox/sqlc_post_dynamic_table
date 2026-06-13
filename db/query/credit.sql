-- name: GetCashCredit :one
select * from cash_credit
where id = @id and name = @name
limit 1;


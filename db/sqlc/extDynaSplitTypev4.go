package sqlc

type tTbGuard map[string]bool

var (
	dynamicTbGuard   = []string{"cash_credit", "cash_inex", "cashflow", "ref_cash", "to_cashflow"}
	cash_creditGuard = tTbGuard{"receivable": true, "payable": true}
	cash_inexGuard   = tTbGuard{"income": true, "expense": true}
	cashflowGuard    = tTbGuard{"income": true, "expense": true, "receivable": true, "payable": true}
	ref_cashGuard    = tTbGuard{"income": true, "expense": true, "receivable": true, "payable": true}
	to_cashflowGuard = tTbGuard{"income": true, "expense": true, "receivable": true, "payable": true}
)

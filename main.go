package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dynagen/gen"
)

func mainBackup() {
	exePath, err := gen.ExePath()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	setting, err := gen.ReadConfig(filepath.Join(exePath, "./sqlc_dyna.json"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = gen.GenerateDynaGenFile(setting, setting.ExtendDynaType)
	if err != nil {
		fmt.Println("cread config error", err)
		os.Exit(1)
	}
	fmt.Println("config created!")

	sqlFiles, err := gen.ReadSqlGoFiles(setting.SQLPath)
	if err != nil {
		fmt.Println("cannot read sql files: ", err)
		os.Exit(1)
	}

	for _, sqlFile := range sqlFiles {
		err = gen.HandleSql(sqlFile, setting)
		if err != nil {
			fmt.Println("err parse sql file: ", err)
			fmt.Println("err parse sql filePath: ", sqlFile)
			os.Exit(1)
		}
	}

	fmt.Println("all sql file processed!")
}

func main() {
	// Sample context parsed from your input configurations
	allowTableGuard := []string{"cashflow", "to_cashflow"}
	queryName := "addCashflowDyna"
	const addCashflowDynaSQL = `
  insert into cashflow(
      id,name
  )select id,name from to_cashflow as c
  where c.id = $1
  returning id
`

	generatedCode, err := gen.GenerateDynamicQueryBuilders(
		queryName,
		addCashflowDynaSQL,
		allowTableGuard,
	)
	if err != nil {
		fmt.Println("Error:")
		return
	}

	fmt.Println(generatedCode)
}

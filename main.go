package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dynagen/gen"
)

func main() {
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

	err = gen.GenerateDynaGenFile(setting, "extDynaType.go")
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

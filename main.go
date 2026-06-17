package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dynagen/gen"
)

func mainBack() {
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

	// fmt.Println("config: ", setting)
	err = gen.CreateExtendType(setting)
	if err != nil {
		fmt.Println("err CreateExtendType: ", err)
		os.Exit(1)
	}

	sqlFiles, err := gen.ReadSqlGoFiles(setting.SQLPath)
	if err != nil {
		fmt.Println("cannot read sql files: ", err)
		os.Exit(1)
	}
	fmt.Println("sql len: ", len(sqlFiles))

	for _, sqlFile := range sqlFiles {

		err = gen.HandleSql(sqlFile, setting)
		if err != nil {
			fmt.Println("err parse sql file: ", err)
			fmt.Println("err parse sql filePath: ", sqlFile)
			os.Exit(1)
		}
	}

	fmt.Println("all sql file processed! 12345")
}

func main() {
	cPath := "/home/ubuntu/projects/go/src/sqlc_post_plugin/sqlc_dyna.json"

	setting, err := gen.ReadConfig(cPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// fmt.Println("config: ", setting)
	err = gen.CreateExtendType(setting)
	if err != nil {
		fmt.Println("err CreateExtendType: ", err)
		os.Exit(1)
	}

	sqlFiles, err := gen.ReadSqlGoFiles(setting.SQLPath)
	if err != nil {
		fmt.Println("cannot read sql files: ", err)
		os.Exit(1)
	}
	fmt.Println("sql len: ", len(sqlFiles))

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

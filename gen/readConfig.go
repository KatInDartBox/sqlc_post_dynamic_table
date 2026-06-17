package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func guardName(n string) string {
	return upperFistStr(n)
}

func tbGuardName(g string) string {
	return "tTb" + g
}

func dynaGuardName(dyna string, g string) string {
	return dyna + g
}

func ReadConfig(configPath string) (*Config, error) {
	// 1. Read the file content
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// 2. Unmarshal the JSON into the struct
	config := Config{
		Name: ConfigName{
			FileTypeName: "extDynaType.go",
			Guard:        "Guard",
			DynaTable:    "dynaTable",
		},
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config JSON: %w", err)
	}

	guard := upperFistStr(config.Name.Guard)
	config.Name.Guard = upperFistStr(guard)
	config.Name.TbGuard = tbGuardName(guard)
	config.Name.DynaTbGuard = dynaGuardName(config.Name.DynaTable, guard)

	config.AllowTableMap = map[string]string{}
	for k := range config.DynaTable {
		lowerK := strings.ToLower(k)
		config.AllowTable = append(config.AllowTable, strings.ToLower(lowerK))
		config.AllowTableMap[lowerK] = toStructName(lowerK)
	}

	return &config, nil
}

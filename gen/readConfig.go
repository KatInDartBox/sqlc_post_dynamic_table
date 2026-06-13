package gen

import (
	"encoding/json"
	"fmt"
	"os"
)

func ReadConfig(configPath string) (*Config, error) {
	// 1. Read the file content
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// 2. Unmarshal the JSON into the struct
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config JSON: %w", err)
	}
	if config.GetDynaQueryTable == "" {
		config.GetDynaQueryTable = "getDynaQueryTable"
	}
	if config.GetDynaQueryFn == "" {
		config.GetDynaQueryFn = "getDynaQuery"
	}

	return &config, nil
}

package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadSqlGoFiles reads the content of all files ending with ".sql.go" in the given folder.
// It returns a map where the key is the filename/filepath and the value is the text file content.
func ReadSqlGoFiles(folderPath string) (files []string, err error) {
	// Read the entries of the directory
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		// Skip directories, we only want files
		if entry.IsDir() {
			continue
		}

		// Check if the filename ends with ".sql.go"
		if strings.HasSuffix(entry.Name(), ".sql.go") {
			fullPath := filepath.Join(folderPath, entry.Name())

			// Read file content safely
			// content, err := os.ReadFile(fullPath)
			// if err != nil {
			// 	return nil, fmt.Errorf("failed to read file %s: %w", entry.Name(), err)
			// }

			files = append(files, fullPath)
		}
	}

	return files, nil
}

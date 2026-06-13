package gen

import (
	"errors"
	"os"
	"path/filepath"
)

func ExePath() (p string, err error) {
	// Get the full path to the executable
	ex, err := os.Executable()
	if err != nil {
		return "", errors.New("could not read exe path")
	}

	// Get the directory containing the executable
	p = filepath.Dir(ex)
	return p, nil
}

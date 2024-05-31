package exporter

import (
	"os"
	"path/filepath"
)

// OutputPath returns absolute path to the output directory (testing_out_stuff).
func OutputPath(dataPath string) string {
	return filepath.Join(os.TempDir(), dataPath)
}

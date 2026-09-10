package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv searches candidate paths for a .env file and loads its key-value pairs
// into the process environment using os.Setenv (if not already set).
// Returns the resolved path of the loaded file, or empty string if none found.
func LoadDotEnv(paths ...string) string {
	candidates := paths
	if len(candidates) == 0 {
		candidates = []string{
			".env",
			"../.env",
			"../../.env",
		}
	}

	for _, path := range candidates {
		cleanPath := filepath.Clean(path)
		file, err := os.Open(cleanPath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Strip matching surrounding quotes
				if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
					val = val[1 : len(val)-1]
				}
				// Set in environment if not already set
				if os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
			}
		}
		return cleanPath
	}

	return ""
}

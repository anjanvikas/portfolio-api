package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE pairs out of path and sets each one in the
// process environment with os.Setenv. Pre-existing env vars are not
// overwritten so a shell-set value always wins.
//
// Format:
//   - lines starting with '#' (after trimming) are comments
//   - blank lines are skipped
//   - VALUE may be surrounded by single or double quotes; the quotes are
//     stripped but no further expansion or escaping is performed
//   - '$' is treated as a literal character (so bcrypt hashes survive)
//
// If the file does not exist, LoadDotEnv returns nil. Any other read error
// is returned to the caller.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, present := os.LookupEnv(key); present {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

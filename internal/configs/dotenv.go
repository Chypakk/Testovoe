package configs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv загружает переменные из .env в окружение процесса.
// Ищет файл в текущей рабочей директории, затем поднимается на несколько
// уровней вверх (на случай запуска из cmd/server). Переменные, уже заданные
// в реальном окружении, не перезаписываются.
func LoadDotEnv(filename string) error {
	path, err := findFile(filename)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Срезаем кавычки, если значение заключено в них
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		} else if key == "JWT_SECRET" || key == "SERVER_PORT" {
			fmt.Fprintf(os.Stderr, "dotenv: %s already set in environment, keeping existing value\n", key)
		}
	}

	return scanner.Err()
}

// findFile ищет файл в текущей директории и в вышестоящих (до 4 уровней).
func findFile(name string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("open %s: file not found", name)
}

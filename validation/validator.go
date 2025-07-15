package validation

import (
	"fmt"
	"regexp"
	"strconv"
)

func countPlaceholders(sql string) int {
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(sql, -1)
	max := 0
	for _, m := range matches {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max
}

func ValidateArgs(sql string, args []any) error {
	maxPlaceholder := countPlaceholders(sql)
	if maxPlaceholder > len(args) {
		return fmt.Errorf("query expects %d arguments but got %d", maxPlaceholder, len(args))
	}
	return nil
}

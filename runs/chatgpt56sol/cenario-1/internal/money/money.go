package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidAmount = errors.New("invalid monetary amount")

// Parse converts a positive decimal amount with at most two fractional digits to cents.
func Parse(value string) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, ErrInvalidAmount
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidAmount
	}

	reais, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}

	var cents int64
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, ErrInvalidAmount
		}
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}

	if reais > (int64(^uint64(0)>>1)-cents)/100 {
		return 0, ErrInvalidAmount
	}
	total := reais*100 + cents
	if total <= 0 {
		return 0, ErrInvalidAmount
	}
	return total, nil
}

func Format(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

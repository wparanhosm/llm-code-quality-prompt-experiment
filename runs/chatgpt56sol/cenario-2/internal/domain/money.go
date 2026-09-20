package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidAmount = errors.New("amount must be a positive decimal with at most two fractional digits")

type Money struct{ cents int64 }

func ParseMoney(value string) (Money, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return Money{}, fmt.Errorf("parse money %q: %w", value, ErrInvalidAmount)
	}
	if parts[0] == "" || len(parts) == 2 && (len(parts[1]) == 0 || len(parts[1]) > 2) {
		return Money{}, fmt.Errorf("parse money %q: %w", value, ErrInvalidAmount)
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1] + strings.Repeat("0", 2-len(parts[1]))
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > (int64(^uint64(0)>>1)-99)/100 {
		return Money{}, fmt.Errorf("parse money %q: %w", value, ErrInvalidAmount)
	}
	frac, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("parse money fraction %q: %w", value, ErrInvalidAmount)
	}
	cents := whole*100 + frac
	if cents <= 0 {
		return Money{}, fmt.Errorf("parse money %q: %w", value, ErrInvalidAmount)
	}
	return Money{cents: cents}, nil
}

func MoneyFromCents(cents int64) Money { return Money{cents: cents} }
func (m Money) Cents() int64           { return m.cents }
func (m Money) String() string         { return fmt.Sprintf("%d.%02d", m.cents/100, m.cents%100) }

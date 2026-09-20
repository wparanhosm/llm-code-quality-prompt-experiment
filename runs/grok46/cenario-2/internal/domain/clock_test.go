package domain

import "testing"

func TestRealClockNow(t *testing.T) {
	t.Parallel()
	c := RealClock{}
	n := c.Now()
	if n.IsZero() {
		t.Fatal("zero time")
	}
}

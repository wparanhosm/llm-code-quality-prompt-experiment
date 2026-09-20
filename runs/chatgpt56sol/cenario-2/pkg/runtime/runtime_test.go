package runtime

import "testing"

func TestRuntime(t *testing.T) {
	id, err := (CryptoIDGenerator{}).New()
	if err != nil || len(id) != 32 {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if (UTCClock{}).Now().Location().String() != "UTC" {
		t.Fatal("not UTC")
	}
}

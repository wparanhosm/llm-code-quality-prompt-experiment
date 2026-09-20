package platform

import "testing"

func TestGetenv(t *testing.T) {
	t.Setenv("P2P_TEST_KEY", "value")
	if Getenv("P2P_TEST_KEY", "def") != "value" {
		t.Fatal("want value")
	}
	if Getenv("P2P_TEST_KEY_MISSING", "def") != "def" {
		t.Fatal("want def")
	}
}

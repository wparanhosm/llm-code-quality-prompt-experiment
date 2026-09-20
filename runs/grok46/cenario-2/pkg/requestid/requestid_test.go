package requestid

import (
	"context"
	"testing"
)

func TestFromContext(t *testing.T) {
	t.Parallel()
	if FromContext(nil) != "" {
		t.Fatal("nil ctx")
	}
	if FromContext(context.Background()) != "" {
		t.Fatal("empty")
	}
	ctx := WithValue(context.Background(), "abc")
	if FromContext(ctx) != "abc" {
		t.Fatal("want abc")
	}
}

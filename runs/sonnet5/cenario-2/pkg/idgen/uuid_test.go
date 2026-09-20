package idgen_test

import (
	"testing"

	"p2p-transfer/pkg/idgen"
)

func TestUUIDGenerator_NewID(t *testing.T) {
	t.Parallel()

	g := idgen.NewUUIDGenerator()

	id1 := g.NewID()
	id2 := g.NewID()

	if id1 == "" {
		t.Fatal("NewID não deveria retornar string vazia")
	}
	if id1 == id2 {
		t.Fatal("chamadas sucessivas de NewID deveriam gerar IDs distintos")
	}
	if len(id1) != 36 {
		t.Fatalf("comprimento do UUID = %d, want 36", len(id1))
	}
}

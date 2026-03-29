package event

import "testing"

func TestHashContentIsStable(t *testing.T) {
	a := HashContent("hello")
	b := HashContent("hello")
	if a != b {
		t.Fatalf("expected stable hash, got %q and %q", a, b)
	}
}

func TestHashContentChangesWithInput(t *testing.T) {
	a := HashContent("hello")
	b := HashContent("world")
	if a == b {
		t.Fatal("expected different hashes for different content")
	}
}

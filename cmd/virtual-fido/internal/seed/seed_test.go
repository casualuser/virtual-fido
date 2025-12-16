package seed

import "testing"

func TestParseSeed(t *testing.T) {
	s, err := parse(" 616263 ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if string(s) != "abc" {
		t.Fatalf("want abc got %q", s)
	}
	if _, err := parse("zz"); err == nil {
		t.Fatalf("expected error for bad hex")
	}
	if _, err := parse("   "); err == nil {
		t.Fatalf("expected error for empty")
	}
}

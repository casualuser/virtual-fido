package airgap

import "testing"

func TestEncodeDecodeRequest(t *testing.T) {
	req := &Request{Op: OpMakeCredential, RPID: "example.com", ClientDataHash: []byte{1, 2, 3}}
	hexStr, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := DecodeRequest(hexStr)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.RPID != req.RPID || out.Op != req.Op {
		t.Fatalf("roundtrip mismatch: %#v", out)
	}
}

func TestSanitizeHex(t *testing.T) {
	s := "  616263\n"
	b, err := sanitizeHex(s)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if string(b) != "abc" {
		t.Fatalf("want abc got %q", b)
	}
	if _, err := sanitizeHex(""); err == nil {
		t.Fatalf("expected error on empty")
	}
	if _, err := sanitizeHex("zz"); err == nil {
		t.Fatalf("expected error on bad hex")
	}
}

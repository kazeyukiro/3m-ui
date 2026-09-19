package hwid

import (
	"net/http"
	"testing"
)

func TestParseRequestValid(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "UE42LJXu4DbiCaBv")
	info := ParseRequest(r)
	if !info.Present || info.HWID != "UE42LJXu4DbiCaBv" {
		t.Fatalf("got %+v", info)
	}
}

func TestParseRequestUUID(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "550e8400-e29b-41d4-a716-446655440000")
	info := ParseRequest(r)
	if !info.Present {
		t.Fatal("expected UUID accepted")
	}
}

func TestParseRequestQueryFallback(t *testing.T) {
	r, _ := http.NewRequest("GET", "/?hwid=abc1234567890", nil)
	info := ParseRequest(r)
	if !info.Present || info.HWID != "abc1234567890" {
		t.Fatalf("got %+v", info)
	}
}

func TestParseRequestShortRejected(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "ab")
	info := ParseRequest(r)
	if info.Present {
		t.Fatal("expected short hwid ignored")
	}
}

func TestParseRequestBase64ish(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "dGVzdC1od2lkL3ZhbHVl+/=xyz")
	info := ParseRequest(r)
	if !info.Present {
		t.Fatal("expected base64-like hwid accepted")
	}
}

func TestNormalizeTruncates(t *testing.T) {
	long := make([]byte, 200)
	for i := range long {
		long[i] = 'a'
	}
	id, ok := normalizeHWID(string(long))
	if !ok || len(id) != maxHWIDLen {
		t.Fatalf("got %q ok=%v", id, ok)
	}
}

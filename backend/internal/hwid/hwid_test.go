package hwid

import (
	"net/http"
	"testing"
)

func TestParseRequestValid(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "UE42LJXu4DbiCaBv")
	r.Header.Set(HeaderDeviceOS, "iOS")
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
		t.Fatal("expected UUID hwid accepted")
	}
}

func TestParseRequestQueryFallback(t *testing.T) {
	r, _ := http.NewRequest("GET", "/?hwid=abc1234567890", nil)
	info := ParseRequest(r)
	if !info.Present || info.HWID != "abc1234567890" {
		t.Fatalf("got %+v", info)
	}
}

func TestParseRequestInvalid(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "short")
	info := ParseRequest(r)
	if info.Present {
		t.Fatal("expected invalid hwid ignored")
	}
}

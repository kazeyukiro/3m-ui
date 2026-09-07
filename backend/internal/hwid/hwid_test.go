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

func TestParseRequestInvalid(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set(HeaderHWID, "short")
	info := ParseRequest(r)
	if info.Present {
		t.Fatal("expected invalid hwid ignored")
	}
}

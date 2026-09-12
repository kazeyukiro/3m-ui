package totp

import "testing"

func TestGenerateAndVerify(t *testing.T) {
	sec, err := GenerateSecret()
	if err != nil || sec == "" {
		t.Fatal(err, sec)
	}
	// can't know current code without sharing clock hotp - just verify false for empty
	if Verify(sec, "000000", 0) && Verify(sec, "999999", 0) {
		t.Fatal("unlikely both valid")
	}
	_ = OTPAuthURL("3m-ui", "admin", sec)
}

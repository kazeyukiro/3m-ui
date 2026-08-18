package telegram

import "testing"

func TestChatAllowlistRequiresExactChatID(t *testing.T) {
	allowed := buildAllowedChats([]string{"12345", "-10067890"})

	for _, id := range []string{"12345", "-10067890"} {
		if !chatAllowed(allowed, id) {
			t.Fatalf("configured chat %q should be allowed", id)
		}
	}

	for _, id := range []string{"-12345", "-10012345", "67890", "-67890", "-100123"} {
		if chatAllowed(allowed, id) {
			t.Fatalf("unconfigured chat %q must not be allowed", id)
		}
	}
}

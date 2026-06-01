package alerter

import "testing"

func TestNormalizeTelegramChatID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1002405693501", "-1002405693501"},
		{"-1002405693501", "-1002405693501"},
		{"123456789", "123456789"},
		{" -10099 ", "-10099"},
	}
	for _, tc := range tests {
		if got := normalizeTelegramChatID(tc.in); got != tc.want {
			t.Fatalf("normalizeTelegramChatID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

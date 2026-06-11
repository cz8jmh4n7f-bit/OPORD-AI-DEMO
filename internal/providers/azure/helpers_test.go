package azure

import "testing"

func TestSplitAzureRuntime(t *testing.T) {
	tests := []struct {
		in      string
		wantRT  string
		wantVer string
	}{
		{"python3.12", "python", "3.12"},
		{"python3.11", "python", "3.11"},
		{"nodejs20.x", "node", "20"},
		{"node20", "node", "20"},
		{"java17", "java", "17"},
		{"dotnet8", "dotnet", "8"},
		{"", "python", "3.12"},        // empty to default
		{"rustfoo", "python", "3.12"}, // unknown to default
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			rt, ver := splitAzureRuntime(tc.in)
			if rt != tc.wantRT || ver != tc.wantVer {
				t.Fatalf("splitAzureRuntime(%q) = (%q, %q), want (%q, %q)", tc.in, rt, ver, tc.wantRT, tc.wantVer)
			}
		})
	}
}

func TestSafePrefix(t *testing.T) {
	tests := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"abc123", 12, "abc123"},
		{"ABC-123", 12, "abc-123"},               // lowercased, hyphen kept
		{"a_b!c@d", 12, "abcd"},                  // invalid chars stripped
		{"verylongworkspaceuuid", 8, "verylong"}, // truncated to maxLen
		{"!!!", 12, "vm"},                        // all-invalid to fallback "vm"
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := safePrefix(tc.in, tc.maxLen); got != tc.want {
				t.Fatalf("safePrefix(%q, %d) = %q, want %q", tc.in, tc.maxLen, got, tc.want)
			}
		})
	}
}

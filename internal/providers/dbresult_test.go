package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDBResult_PasswordNotSerialized guards the no-leak invariant: the managed-DB
// master password (Cloud SQL / Azure Flexible Server must set one at create time)
// is carried on DBResult.Password only to hand to the secrets store - it must NEVER
// be serialized into resources.observed. The `json:"-"` tag enforces this even if a
// caller forgets to clear the field.
func TestDBResult_PasswordNotSerialized(t *testing.T) {
	b, err := json.Marshal(DBResult{
		Endpoint:   "db.example:5432",
		Port:       5432,
		Password:   "super-secret-pw",
		RawOutputs: map[string]any{"endpoint": "db.example:5432"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "super-secret-pw") {
		t.Fatalf("password value leaked into serialized DBResult: %s", s)
	}
	if strings.Contains(strings.ToLower(s), "password") {
		t.Fatalf("a password field was serialized into DBResult: %s", s)
	}
}

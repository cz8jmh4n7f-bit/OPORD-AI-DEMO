package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// VaultKV is a KV v2 store backed by Vault/OpenBao, using the check-and-set
// (cas) option for atomic writes. The pool secret lives at
// <mount>/data/<path> with fields "<cidr>" => "<owner>".
type VaultKV struct {
	client *vault.Client
	mount  string
	path   string
}

// NewVaultPool builds an IPAM Pool backed by a Vault KV v2 secret.
func NewVaultPool(addr, token, mount, path string, log *slog.Logger) (*Pool, error) {
	if addr == "" || token == "" {
		return nil, fmt.Errorf("ipam: vault addr and token are required")
	}
	cfg := vault.DefaultConfig()
	cfg.Address = addr
	c, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("ipam: vault client: %w", err)
	}
	c.SetToken(token)
	return New(&VaultKV{client: c, mount: mount, path: path}, log), nil
}

func (v *VaultKV) Read(ctx context.Context) (map[string]string, int, error) {
	sec, err := v.client.Logical().ReadWithContext(ctx, v.mount+"/data/"+v.path)
	if err != nil {
		return nil, 0, err
	}
	pool := map[string]string{}
	if sec == nil || sec.Data == nil {
		return pool, 0, nil
	}
	if data, ok := sec.Data["data"].(map[string]any); ok {
		for k, val := range data {
			s, _ := val.(string)
			pool[k] = s
		}
	}
	version := 0
	if md, ok := sec.Data["metadata"].(map[string]any); ok {
		version = toInt(md["version"])
	}
	return pool, version, nil
}

func (v *VaultKV) CASWrite(ctx context.Context, pool map[string]string, expected int) (bool, error) {
	payload := map[string]any{
		"data":    pool,
		"options": map[string]any{"cas": expected},
	}
	_, err := v.client.Logical().WriteWithContext(ctx, v.mount+"/data/"+v.path, payload)
	if err != nil {
		// KV v2 returns a 400 "check-and-set parameter did not match" on a stale
		// version - treat that as a (retryable) conflict, not a hard error.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "check-and-set") || strings.Contains(msg, "did not match") {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// toInt coerces a JSON number (json.Number or float64) to int.
func toInt(v any) int {
	switch n := v.(type) {
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

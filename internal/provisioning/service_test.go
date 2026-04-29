package provisioning

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRandomLaravelAppKey(t *testing.T) {
	t.Parallel()

	key, err := randomLaravelAppKey()
	if err != nil {
		t.Fatalf("random laravel app key: %v", err)
	}

	encoded, ok := strings.CutPrefix(key, "base64:")
	if !ok {
		t.Fatalf("key = %q, want base64 prefix", key)
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded key length = %d, want 32", len(decoded))
	}
}

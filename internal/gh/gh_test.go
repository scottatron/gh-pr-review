package gh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentPrNumber(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setupFakeGh(t, `#!/bin/sh
if [ "$1" = "pr" ] && [ "$2" = "view" ]; then
  echo '{"number":123}'
  exit 0
fi
exit 1
`)

		ctx := context.Background()
		number, err := CurrentPrNumber(ctx)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if number != 123 {
			t.Fatalf("expected number 123, got %d", number)
		}
	})

	t.Run("empty-number", func(t *testing.T) {
		setupFakeGh(t, `#!/bin/sh
echo '{"number":0}'
`)

		_, err := CurrentPrNumber(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid-json", func(t *testing.T) {
		setupFakeGh(t, `#!/bin/sh
echo 'not-json'
`)

		_, err := CurrentPrNumber(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func setupFakeGh(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	return path
}

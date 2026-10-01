//go:build !windows

package discover

import (
	"os"
	"testing"
)

// lockDirListing removes all permission bits so filepath.Walk cannot list the
// directory. Restores 0755 on cleanup so TempDir removal still works.
func lockDirListing(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	// Some filesystems ignore mode bits; skip rather than false-fail.
	if f, err := os.Open(dir); err == nil {
		_ = f.Close()
		t.Skip("filesystem does not honor directory mode bits")
	}
}

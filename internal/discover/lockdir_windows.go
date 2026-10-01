//go:build windows

package discover

import (
	"os"
	"os/exec"
	"testing"
)

// lockDirListing denies list/read for the current user via NTFS ACL.
// Unix mode bits are ignored on NTFS, so chmod alone cannot exercise the
// unreadable-root path that FindGitRepos must treat as an error.
func lockDirListing(t *testing.T, dir string) {
	t.Helper()
	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("USERNAME unset; cannot set NTFS deny ACE")
	}
	// (RX) blocks open/list; WRITE_DAC is left alone so cleanup can undo this.
	out, err := exec.Command("icacls", dir, "/deny", user+":(RX)").CombinedOutput()
	if err != nil {
		t.Skipf("icacls deny failed: %v (%s)", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("icacls", dir, "/remove:d", user).Run()
		_ = exec.Command("icacls", dir, "/grant", user+":(OI)(CI)F").Run()
	})
	if f, err := os.Open(dir); err == nil {
		_ = f.Close()
		t.Skip("directory still listable after NTFS deny ACE")
	}
}

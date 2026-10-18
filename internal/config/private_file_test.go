package config

import (
	"os"
	"syscall"
	"testing"
	"time"
)

type secretFileInfo struct {
	mode os.FileMode
	sys  any
}

func (info secretFileInfo) Name() string       { return "secret" }
func (info secretFileInfo) Size() int64        { return 0 }
func (info secretFileInfo) Mode() os.FileMode  { return info.mode }
func (info secretFileInfo) ModTime() time.Time { return time.Time{} }
func (info secretFileInfo) IsDir() bool        { return false }
func (info secretFileInfo) Sys() any           { return info.sys }

// TestPrivateSecretFileRequiresCurrentOwnerAndPrivateRegularFile migrates the
// former boolean helper to the classifier: every failing predicate is reported
// with its precise sub-shape, and a passing file reports no cause at all. The
// predicates and their order are unchanged and fail closed; the classifier
// never reports a UID or mode value, only the finite cause class.
func TestPrivateSecretFileRequiresCurrentOwnerAndPrivateRegularFile(t *testing.T) {
	uid := uint32(os.Geteuid())
	for _, tc := range []struct {
		name  string
		info  secretFileInfo
		shape string
	}{
		{"current owner", secretFileInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: uid}}, ""},
		{"foreign owner", secretFileInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: uid + 1}}, "wrong_owner"},
		{"group readable", secretFileInfo{mode: 0o640, sys: &syscall.Stat_t{Uid: uid}}, "not_private"},
		{"symlink", secretFileInfo{mode: os.ModeSymlink | 0o600, sys: &syscall.Stat_t{Uid: uid}}, "symlink"},
		{"directory", secretFileInfo{mode: os.ModeDir | 0o700, sys: &syscall.Stat_t{Uid: uid}}, "non_regular"},
		{"world readable", secretFileInfo{mode: 0o644, sys: &syscall.Stat_t{Uid: uid}}, "not_private"},
		{"owner unverifiable", secretFileInfo{mode: 0o600, sys: struct{}{}}, "owner_unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := secretFilePolicy(tc.info); got != tc.shape {
				t.Fatalf("secretFilePolicy()=%q, want %q", got, tc.shape)
			}
		})
	}
	// A nil FileInfo is a non-regular sub-shape, not a fake policy pass.
	if got := secretFilePolicy(nil); got != "non_regular" {
		t.Fatalf("secretFilePolicy(nil)=%q, want %q", got, "non_regular")
	}
}

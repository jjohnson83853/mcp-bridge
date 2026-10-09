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

func TestPrivateSecretFileRequiresCurrentOwnerAndPrivateRegularFile(t *testing.T) {
	uid := uint32(os.Geteuid())
	for _, tc := range []struct {
		name string
		info secretFileInfo
		want bool
	}{
		{"current owner", secretFileInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: uid}}, true},
		{"foreign owner", secretFileInfo{mode: 0o600, sys: &syscall.Stat_t{Uid: uid + 1}}, false},
		{"group readable", secretFileInfo{mode: 0o640, sys: &syscall.Stat_t{Uid: uid}}, false},
		{"not regular", secretFileInfo{mode: os.ModeSymlink | 0o600, sys: &syscall.Stat_t{Uid: uid}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := privateSecretFile(tc.info); got != tc.want {
				t.Fatalf("privateSecretFile()=%v, want %v", got, tc.want)
			}
		})
	}
}

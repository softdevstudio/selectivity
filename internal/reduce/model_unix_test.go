//go:build !windows

package reduce

import (
	"os"
	"syscall"
	"testing"
)

func TestCopiedMode(t *testing.T) {
	// Tests in this package are sequential. Always restore the process-wide mask.
	mask := syscall.Umask(0077)
	defer syscall.Umask(mask)
	src, out := setup(t, "BUG\n")
	if e := os.Chmod(src, 0777); e != nil {
		t.Fatal(e)
	}
	in, e := Load(src)
	if e != nil {
		t.Fatal(e)
	}
	if e = in.Write(out); e != nil {
		t.Fatal(e)
	}
	i, e := os.Stat(out)
	if e != nil {
		t.Fatal(e)
	}
	if i.Mode().Perm() != 0777 {
		t.Fatalf("mode %o", i.Mode().Perm())
	}
}

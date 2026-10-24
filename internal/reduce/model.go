package reduce

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type File struct {
	Name string
	Data []byte
	Mode fs.FileMode
}
type Input struct {
	Directory bool
	Files     []File
	Dirs      []string
}

func Load(path string) (Input, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Input{}, err
	}
	if info.Mode().IsRegular() {
		b, e := os.ReadFile(path)
		return Input{Files: []File{{"input", b, info.Mode().Perm()}}}, e
	}
	if !info.IsDir() {
		return Input{}, fmt.Errorf("input must be a regular file or directory")
	}
	in := Input{Directory: true}
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if p != path {
				n, e := filepath.Rel(path, p)
				if e != nil {
					return e
				}
				in.Dirs = append(in.Dirs, n)
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is unsupported: %s", p)
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		if !i.Mode().IsRegular() {
			return fmt.Errorf("special file is unsupported: %s", p)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		n, e := filepath.Rel(path, p)
		if e != nil {
			return e
		}
		in.Files = append(in.Files, File{n, b, i.Mode().Perm()})
		return nil
	})
	sort.Slice(in.Files, func(i, j int) bool { return in.Files[i].Name < in.Files[j].Name })
	return in, err
}

// Write always recreates data from the in-memory snapshot. Predicate mutations
// cannot become accepted input. The destination must be inside our private run.
func (in Input) Write(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if !in.Directory {
		if len(in.Files) != 1 {
			return fmt.Errorf("file input has no file")
		}
		return writeFile(path, in.Files[0])
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	for _, d := range in.Dirs {
		if err := os.MkdirAll(filepath.Join(path, d), 0700); err != nil {
			return err
		}
	}
	for _, f := range in.Files {
		p := filepath.Join(path, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		if err := writeFile(p, f); err != nil {
			return err
		}
	}
	return nil
}
func writeFile(path string, f File) error {
	if err := os.WriteFile(path, f.Data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, f.Mode)
}
func (in Input) Clone() Input { out := in; out.Files = append([]File(nil), in.Files...); return out }
func (in Input) Bytes() int64 {
	var n int64
	for _, f := range in.Files {
		n += int64(len(f.Data))
	}
	return n
}
func (in Input) Hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "dir=%t\n", in.Directory)
	for _, d := range in.Dirs {
		fmt.Fprintf(h, "directory:%d:%s\n", len(d), d)
	}
	for _, f := range in.Files {
		fmt.Fprintf(h, "%d:%s:%d:%d:", len(f.Name), f.Name, f.Mode, len(f.Data))
		h.Write(f.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func Lines(b []byte) [][]byte {
	if len(b) == 0 {
		return nil
	}
	a := bytes.SplitAfter(b, []byte{'\n'})
	if len(a[len(a)-1]) == 0 {
		a = a[:len(a)-1]
	}
	return a
}

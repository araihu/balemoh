package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// ContentDigest pins all regular package files, names and executable modes.
// Symlinks and packages beyond 128 files or 256 MiB are rejected.
func (p Package) ContentDigest() (string, error) {
	root, err := os.OpenRoot(p.Root)
	if err != nil {
		return "", errors.New("invalid package")
	}
	defer root.Close()
	h := sha256.New()
	var total int64
	count := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || count > 128 || info.Size() > (256<<20)-total {
			return errors.New("invalid package contents")
		}
		total += info.Size()
		fmt.Fprintf(h, "%d:%s:%o:%d:", len(name), name, info.Mode().Perm(), info.Size())
		f, err := root.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(h, io.LimitReader(f, info.Size()+1))
		if err != nil || n != info.Size() {
			return errors.New("package changed while reading")
		}
		return nil
	})
	if err != nil {
		return "", errors.New("invalid package contents")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package deps

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Digest hashes a package directory's contents (design D5): every regular
// file under the root, relative slash paths sorted, each folded into the
// hash as path bytes + 0x00 + content bytes + 0x00. The rendering is
// "sha256-" + lowercase hex. Fixed content, fixed digest — the goldens
// assert the exact string.
func Digest(pkgDir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(pkgDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, rerr := filepath.Rel(pkgDir, path)
		if rerr != nil {
			return rerr
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, rel := range paths {
		h.Write([]byte(rel))
		h.Write([]byte{0})
		data, err := os.ReadFile(filepath.Join(pkgDir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		h.Write(data)
		h.Write([]byte{0})
	}
	return "sha256-" + hex.EncodeToString(h.Sum(nil)), nil
}

// Acquire copies one package version from the registry tree into the
// cache, mirroring <name>/<version>/ whole (files 0o644, directories
// 0o755). An already-present copy is left as is — acquisition is
// idempotent, and on the lock-win path the cache is never rewritten.
func Acquire(regRoot, cacheRoot, name string, v Version) error {
	src := filepath.Join(regRoot, name, v.String())
	dst := filepath.Join(cacheRoot, name, v.String())
	return copyTree(src, dst)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// CacheRoot resolves WE_CACHE, defaulting to the user cache dir's
// we/packages (a cross-project shared layer — chapter 22 R6's we clean
// posture presumes the cache lives outside any one project).
func CacheRoot() (string, error) {
	if v := os.Getenv("WE_CACHE"); v != "" {
		return v, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "we", "packages"), nil
}

// RegistryRoot resolves WE_REGISTRY; unset or absent means an empty source
// universe (every name then answers to nothing).
func RegistryRoot() string {
	return os.Getenv("WE_REGISTRY")
}

// PackageDir is one package version's root inside a source tree.
func PackageDir(root, name string, v Version) string {
	return filepath.Join(root, name, v.String())
}

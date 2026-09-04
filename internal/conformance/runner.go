// Package conformance runs golden CLI cases: each case file under
// testdata/cases is one argv vector plus the expected exit code, stream
// outputs, and (for we new) the created file tree. Expectations are pinned
// by hand from the spec; WE_UPDATE_GOLDEN=1 regenerates them from actual
// behavior as a maintenance tool, never as the initial source of truth.
package conformance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ltlvtao/welang/internal/cli"
)

// Setup prepares the case's working directory before the run: directories
// first, then files (so a file may live inside a setup directory).
type Setup struct {
	Dirs  []string          `json:"dirs,omitempty"`
	Files map[string]string `json:"files,omitempty"`
}

// Case is one golden CLI case.
type Case struct {
	Name   string            `json:"name"`
	Args   []string          `json:"args"`
	Setup  *Setup            `json:"setup,omitempty"`
	Exit   int               `json:"exit"`
	Stdout string            `json:"stdout"`
	Stderr string            `json:"stderr"`
	Files  map[string]string `json:"files,omitempty"`
}

// Result is what the run actually produced.
type Result struct {
	Exit   int
	Stdout string
	Stderr string
	Files  map[string]string
}

// Execute runs one case with workdir as the process working directory,
// restoring the previous directory afterwards. Paths in the captured output
// are normalized: the absolute workdir renders as `<dir>`.
func Execute(c *Case, workdir string) Result {
	prev, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(workdir); err != nil {
		panic(err)
	}
	for _, d := range c.Setup.GetDirs() {
		if err := os.MkdirAll(filepath.FromSlash(d), 0o755); err != nil {
			panic(err)
		}
	}
	for _, f := range c.Setup.SortedFiles() {
		p := filepath.FromSlash(f.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
			panic(err)
		}
	}
	var out, errb bytes.Buffer
	code := cli.Run(c.Args, &out, &errb)
	res := Result{
		Exit:   code,
		Stdout: normalize(out.String(), workdir),
		Stderr: normalize(errb.String(), workdir),
		Files:  snapshotTree(workdir),
	}
	if err := os.Chdir(prev); err != nil {
		panic(err)
	}
	return res
}

// GetDirs returns setup directories (nil-safe).
func (s *Setup) GetDirs() []string {
	if s == nil {
		return nil
	}
	return s.Dirs
}

// SetupFile is one setup file's slash-separated path and exact content.
type SetupFile struct {
	Path    string
	Content string
}

// SortedFiles returns setup files sorted by path, so setup is deterministic.
func (s *Setup) SortedFiles() []SetupFile {
	if s == nil {
		return nil
	}
	paths := make([]string, 0, len(s.Files))
	for p := range s.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	files := make([]SetupFile, 0, len(paths))
	for _, p := range paths {
		files = append(files, SetupFile{Path: p, Content: s.Files[p]})
	}
	return files
}

// LoadCases reads every case file in dir, sorted by name.
func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	cases := make([]Case, 0, len(names))
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		var c Case
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// WriteCase serializes a case back to its file, preserving field order by
// construction (encoding/json emits struct fields in declaration order).
func WriteCase(path string, c Case) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// snapshotTree walks dir and returns every regular file by slash-separated
// relative path. It observes created files only; permissions and other
// metadata are not part of the golden surface.
func snapshotTree(dir string) map[string]string {
	files := map[string]string{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	return files
}

func normalize(s, workdir string) string {
	return strings.ReplaceAll(s, workdir, "<dir>")
}

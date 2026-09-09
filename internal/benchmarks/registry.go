package benchmarks

import (
	"embed"
	"io/fs"
	"sort"
	"sync"
)

// The seed task set (docs/benchmarks.md, "Task taxonomy"): seven
// root-cause axes times two tasks each plus four reverse controls. Each
// task directory carries prompt.md (the model-facing contract, English),
// traps.md (the scoring face: trap list with detection paths), and
// reference/ — a complete We project (we.toml, src/, tests/) whose
// solutions are self-certified green by the package tests.
//
//go:embed all:tasks
var tasksFS embed.FS

// Task is one seed task of the evaluation suite.
type Task struct {
	// ID is the task's kebab identifier and directory name (race-01).
	ID string
	// Prompt is the model-facing requirement text (prompt.md, English).
	Prompt string
	// Traps is the scoring-face trap list with detection paths (traps.md).
	Traps string
	// Files is the reference project: slash paths to exact contents
	// (we.toml, src/main.we, tests/...).
	Files map[string]string
}

var (
	tasksOnce sync.Once
	taskList  []Task
	taskIndex map[string]Task
)

func parseTasks() {
	tasksOnce.Do(func() {
		entries, err := fs.ReadDir(tasksFS, "tasks")
		if err != nil {
			panic(err)
		}
		taskIndex = make(map[string]Task)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			id := e.Name()
			t := Task{ID: id, Files: make(map[string]string)}
			prompt, err := fs.ReadFile(tasksFS, "tasks/"+id+"/prompt.md")
			if err != nil {
				panic(err)
			}
			t.Prompt = string(prompt)
			traps, err := fs.ReadFile(tasksFS, "tasks/"+id+"/traps.md")
			if err != nil {
				panic(err)
			}
			t.Traps = string(traps)
			ref := "tasks/" + id + "/reference"
			walkErr := fs.WalkDir(tasksFS, ref, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				content, err := fs.ReadFile(tasksFS, p)
				if err != nil {
					return err
				}
				t.Files[p[len(ref)+1:]] = string(content)
				return nil
			})
			if walkErr != nil {
				panic(walkErr)
			}
			taskList = append(taskList, t)
			taskIndex[id] = t
		}
		sort.Slice(taskList, func(i, j int) bool { return taskList[i].ID < taskList[j].ID })
	})
}

// Tasks returns the seed task set, sorted by ID. The list is parsed once
// from the embedded tree.
func Tasks() []Task {
	parseTasks()
	return taskList
}

// Has reports whether id names a task in the set — the task-set face
// LoadBatch validates batch task ids against.
func Has(id string) bool {
	parseTasks()
	_, ok := taskIndex[id]
	return ok
}

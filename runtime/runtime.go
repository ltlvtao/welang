// Package weruntime embeds the runtime's C sources so a built we binary
// carries them (go:embed is the only Go mechanism binding sources to the
// binary). The C files live in c/, a directory with no Go files: the go
// tool rejects .c sources in a non-cgo package directory, and only a
// package-less directory keeps them invisible to it. `we build`
// materializes them under build/ and compiles them with the pinned clang
// (design D6 of the native-vertical change); the prebuilt-objects
// distribution story arrives with packaging. Since M8 the allocator pair
// lives in gc.c (alloc.c's malloc pass-through retired — the ABI stayed,
// the body became the collector's).
package weruntime

import _ "embed"

//go:embed c/startup.c
var StartupSource string

//go:embed c/gc.c
var GCSource string

//go:embed c/io.c
var IOSource string

//go:embed c/sched.h
var SchedHeader string

//go:embed c/sched.c
var SchedSource string

//go:embed c/conc.c
var ConcSource string

//go:embed c/test.c
var TestSource string

//go:embed c/str.h
var StrHeader string

//go:embed c/str.c
var StrSource string

//go:embed c/list.h
var ListHeader string

//go:embed c/list.c
var ListSource string

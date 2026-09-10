package benchmarks

import (
	"strings"
	"testing"
)

// Trap calibration (design D3/D6, tasks.md T5): every machine-calibratable
// trap of every task's traps.md carries at least one caught negative here.
// Each case mutates the reference solution into the trap's typical wrong
// shape and asserts the exact bucket the verdict machine assigns — check
// rejections (rejected), reference-test failures (latent), honest run
// declines (test-malformed / boundary). Deadlock-shaped traps abort
// deterministically under the virtual clock in-process, so they calibrate
// without a wall clock; the single face that cannot calibrate here is the
// busy-loop hang (no wait source) — disclosed in control-01's traps.md and
// in the runner's file header.
//
// Disclosed non-faces, listed where they fall:
//   - control-01 "forgetting to advance the loop counter" — no verdict face
//     in-process (the run never parks); needs the real-batch wall-clock
//     guard.
//   - ctxpass-01 "recomputing base from module-level state" — bait closed
//     by design (E0403), a design note, not a trap.
//   - implconv-01 "passing a narrow value at the call sites" — the call
//     sites live in the test face, which attempts cannot touch; the
//     detection path (E0501) is calibrated three times below.
//   - nullbnd-01 "draining a fixed count" and control-02 "swapping the
//     roles" alone — behaviorally dead / correct shapes; the calibrated
//     faces are the missing close and the swap-plus-reset compound.
//   - race-01 "interleaved unsynchronized increments" — not expressible
//     without tripping E1603/E1602 first (the design closes the bait).
//   - resleak-02 "acquire then set 7 last without scheduling the release"
//     — observably identical at L1, disclosed.
//   - errpath-02's implementation note — a compiler-defect disclosure, not
//     a model trap.

// rep is a single-anchor replacement that fails loudly when the mutation
// anchor drifts out of the reference text.
func rep(src, old, new string) string {
	if !strings.Contains(src, old) {
		panic("calibration: mutation anchor missing: " + old)
	}
	return strings.Replace(src, old, new, 1)
}

// repx is rep for anchors that must replace every occurrence.
func repx(src, old, new string) string {
	if !strings.Contains(src, old) {
		panic("calibration: mutation anchor missing: " + old)
	}
	return strings.ReplaceAll(src, old, new)
}

func TestTrapCalibration(t *testing.T) {
	cases := []struct {
		task string
		trap string
		want Bucket
		mut  func(src string) string
	}{
		// ---- control-01 (seriesSum) ----
		{"control-01", "off-by-one-bounds", BucketLatent, func(s string) string {
			return rep(s, "while i <= n {", "while i < n {")
		}},
		{"control-01", "narrow-counter-e0501", BucketRejected, func(s string) string {
			return rep(s, "var i: Int64 = 1", "var i: Int32 = 1")
		}},
		// ---- control-02 (slowProduct) ----
		{"control-02", "inner-reset-outside", BucketLatent, func(s string) string {
			return rep(s, "    while i < a {\n        var j: Int64 = 0\n        while j < b {",
				"    var j: Int64 = 0\n    while i < a {\n        while j < b {")
		}},
		{"control-02", "swap-roles-plus-reset", BucketLatent, func(s string) string {
			s = rep(s, "while i < a {", "while i < b {")
			s = rep(s, "while j < b {", "while j < a {")
			return rep(s, "    while i < b {\n        var j: Int64 = 0",
				"    var j: Int64 = 0\n    while i < b {")
		}},
		{"control-02", "narrow-counter-e0501", BucketRejected, func(s string) string {
			return rep(s, "var i: Int64 = 0", "var i: Int32 = 0")
		}},
		// ---- control-03 (tag/echo) ----
		{"control-03", "concatenation-boundary", BucketBoundary, func(s string) string {
			return rep(s, "    return s", `    return s + "x"`)
		}},
		{"control-03", "string-equality-decline", BucketTestMalformed, func(s string) string {
			return rep(s, `pub fn echo(s: String) -> String {
    return s
}`, `pub fn echo(s: String) -> String {
    var r: String = s
    if s == "mirror" {
        r = "mirror-x"
    }
    return r
}`)
		}},
		{"control-03", "wrong-literal", BucketLatent, func(s string) string {
			return rep(s, `return "we-bench"`, `return "we-bench!"`)
		}},
		// ---- control-04 (parityClass) ----
		{"control-04", "modulo-decline", BucketTestMalformed, func(s string) string {
			return rep(s, `pub fn parityClass(n: Int64) -> Int64 {
    var m: Int64 = n
    while m >= 2 {
        m = m - 2
    }
    return m
}`, `pub fn parityClass(n: Int64) -> Int64 {
    return n % 2
}`)
		}},
		{"control-04", "loop-condition-off-by-one", BucketLatent, func(s string) string {
			return rep(s, "while m >= 2 {", "while m > 2 {")
		}},
		// control-04's "mutating the parameter binding" case retired with
		// the T2-a statement set (codegen-mono design D11): assigning a
		// let or a parameter emits the value-correct equivalent form, so
		// the mutated reference is a clean program, not a trap. The
		// task's traps.md entry went with it. What remains calibrated for
		// control-04 is the modulo decline above and the off-by-one below.
		// ---- ctxpass-01 (scaledMerge) ----
		{"ctxpass-01", "task-captures-var-e1603", BucketRejected, func(s string) string {
			return rep(s, "    let b = base", "    var b: Int64 = base")
		}},
		{"ctxpass-01", "record-capture-e1602", BucketRejected, func(s string) string {
			s = rep(s, "pub type AppError = Failed(String)",
				"pub type AppError = Failed(String)\n\nrecord Cfg { s: Int64 }")
			s = rep(s, "    let s = 3", "    let cfg = Cfg { s: 3 }")
			return repx(s, "ch.send(b + s)", "ch.send(b + cfg.s)")
		}},
		{"ctxpass-01", "forgotten-join-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = c.await()\n", "")
		}},
		// ---- ctxpass-02 (scaled) ----
		{"ctxpass-02", "worker-captures-var-e1603", BucketRejected, func(s string) string {
			return rep(s, "Some(x) => { out.send(x * 2) }", "Some(x) => { out.send(x * 2 + result) }")
		}},
		{"ctxpass-02", "never-send-config-deadlock", BucketLatent, func(s string) string {
			return rep(s, "        cfg.send(base + 1)\n", "")
		}},
		{"ctxpass-02", "read-reply-before-scope-deadlock", BucketLatent, func(s string) string {
			return rep(s, "    var result: Int64 = 0\n    scope {",
				"    var result: Int64 = 0\n    let _ = out.receive()\n    scope {")
		}},
		{"ctxpass-02", "forgotten-join-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = t.await()\n", "")
		}},
		// ---- dangling-01 (joinedSum) ----
		{"dangling-01", "unawaited-handle-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = b.await()\n", "")
		}},
		{"dangling-01", "spawn-outside-scope-e1618", BucketRejected, func(s string) string {
			return rep(s, `    scope {
        let a = task effect io {
            ch.send(5)
        }
        let b = task effect io {
            ch.send(6)
        }
        let _ = a.await()
        let _ = b.await()
    }`, `    let a = task effect io {
        ch.send(5)
    }
    let b = task effect io {
        ch.send(6)
    }
    scope {
        let _ = a.await()
        let _ = b.await()
    }`)
		}},
		{"dangling-01", "handle-escape-e1607", BucketRejected, func(s string) string {
			s = rep(s, "        let _ = a.await()\n        let _ = b.await()\n    }",
				"        let _ = b.await()\n    }")
			return rep(s, "    var total: Int64 = 0\n    let v1 = ch.receive()",
				"    var total: Int64 = 0\n    let av = a.await()\n    let _ = av\n    let v1 = ch.receive()")
		}},
		{"dangling-01", "byres-alias-e1105", BucketRejected, func(s string) string {
			s = rep(s, "pub type AppError = Failed(String)",
				"pub type AppError = Failed(String)\n\nbyres record FileHandle { fd: Int64 }\nimpl Releasable for FileHandle {\n    fn release(mut self) {\n        return\n    }\n}\n\nfn openHandle() -> FileHandle {\n    return FileHandle { fd: 1 }\n}")
			return rep(s, "    let ch: conc.Channel<Int64> = conc.channel(2)",
				"    let ch: conc.Channel<Int64> = conc.channel(2)\n    let f = openHandle()\n    let g = f")
		}},
		// ---- dangling-02 (jobResult) ----
		{"dangling-02", "bare-int-await-e0501", BucketRejected, func(s string) string {
			return rep(s, "        let r = t.await()", "        let n: Int64 = t.await()")
		}},
		{"dangling-02", "dropped-await-result", BucketLatent, func(s string) string {
			return rep(s, `        let r = t.await()
        match r {
            Ok(v) => { out = v }
            Err(_) => { out = 0 }
        }`, "        let _ = t.await()")
		}},
		{"dangling-02", "only-ok-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "            Err(_) => { out = 0 }\n", "")
		}},
		{"dangling-02", "blind-q-e1202", BucketRejected, func(s string) string {
			return rep(s, `        let r = t.await()
        match r {
            Ok(v) => { out = v }
            Err(_) => { out = 0 }
        }`, "        out = t.await()?")
		}},
		{"dangling-02", "no-await-e1607", BucketRejected, func(s string) string {
			return rep(s, `        let r = t.await()
        match r {
            Ok(v) => { out = v }
            Err(_) => { out = 0 }
        }
`, "")
		}},
		// ---- errpath-01 (emptyWait) ----
		{"errpath-01", "plain-receive-deadlock", BucketLatent, func(s string) string {
			return rep(s, `    let r = scope timeout(50) {
        let t = task effect io {
            let v = ch.receive()
            match v {
                Some(x) => { let _ = x }
                None => { let _ = 0 }
            }
        }
        let _ = t.await()
    }
    match r {
        Ok(_) => { out = 1 }
        Err(_) => { out = 100 }
    }`, `    let v = ch.receive()
    match v {
        Some(x) => { let _ = x }
        None => { let _ = 0 }
    }`)
		}},
		{"errpath-01", "dropped-scope-result-e0605", BucketRejected, func(s string) string {
			return rep(s, "    let r = scope timeout(50) {", "    scope timeout(50) {")
		}},
		{"errpath-01", "only-ok-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "        Err(_) => { out = 100 }\n", "")
		}},
		{"errpath-01", "blind-r-e1202", BucketRejected, func(s string) string {
			return rep(s, `    match r {
        Ok(_) => { out = 1 }
        Err(_) => { out = 100 }
    }`, "    let v = r?")
		}},
		{"errpath-01", "own-fiber-receive-deadlock", BucketLatent, func(s string) string {
			return rep(s, `    let r = scope timeout(50) {
        let t = task effect io {
            let v = ch.receive()
            match v {
                Some(x) => { let _ = x }
                None => { let _ = 0 }
            }
        }
        let _ = t.await()
    }`, `    let r = scope timeout(50) {
        let v = ch.receive()
        match v {
            Some(x) => { let _ = x }
            None => { let _ = 0 }
        }
    }`)
		}},
		// ---- errpath-02 (drainOrTimeout) ----
		{"errpath-02", "dry-direct-receive-deadlock", BucketLatent, func(s string) string {
			return rep(s, `    let r = scope timeout(50) {
        let t = task effect io {
            let v = dry.receive()
            match v {
                Some(x) => { let _ = x }
                None => { let _ = 0 }
            }
        }
        let _ = t.await()
    }
    match r {
        Ok(_) => { flag = 1 }
        Err(_) => { flag = 1000 }
    }`, `    let second = dry.receive()
    match second {
        Some(x) => { flag = 1 }
        None => { flag = 1000 }
    }`)
		}},
		{"errpath-02", "dropped-scope-result-e0605", BucketRejected, func(s string) string {
			return rep(s, "    let r = scope timeout(50) {", "    scope timeout(50) {")
		}},
		{"errpath-02", "missing-err-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "        Err(_) => { flag = 1000 }\n", "")
		}},
		{"errpath-02", "closed-dry-value-break", BucketLatent, func(s string) string {
			return rep(s, "    loaded.send(7)", "    loaded.send(7)\n    dry.close()")
		}},
		// ---- implconv-01 (packetScore) ----
		{"implconv-01", "narrow-accumulator-e0501", BucketRejected, func(s string) string {
			return rep(s, "    var t: Int64 = base", "    var t: Int32 = base")
		}},
		{"implconv-01", "mixed-widths-e0501", BucketRejected, func(s string) string {
			return rep(s, "t = t + 3", "t = t + 3i32")
		}},
		{"implconv-01", "widening-return-e0501", BucketRejected, func(s string) string {
			return rep(s, `pub fn packetScore(base: Int64) -> Int64 {
    var t: Int64 = base
    t = t + 3
    if t > 10 {
        t = t + 100
    }
    return t
}`, `pub fn packetScore(base: Int64) -> Int64 {
    var t: Int32 = 8i32
    return t
}`)
		}},
		// ---- implconv-02 (gate) ----
		{"implconv-02", "bare-literal-e0501", BucketRejected, func(s string) string {
			return rep(s, "acc = acc + 3i32", "acc = acc + 3")
		}},
		{"implconv-02", "narrow-return-e0501", BucketRejected, func(s string) string {
			return rep(s, "    return out", "    return acc")
		}},
		{"implconv-02", "wide-accumulator-e0501", BucketRejected, func(s string) string {
			return rep(s, "    var acc: Int32 = seed", "    var acc: Int64 = seed")
		}},
		{"implconv-02", "cross-width-compare-e0501", BucketRejected, func(s string) string {
			return rep(s, "if acc == 5i32", "if acc == 5")
		}},
		// ---- nullbnd-01 (countAbove) ----
		{"nullbnd-01", "only-some-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "        None => { count = count + 0 }\n    }\n    return count",
				"    }\n    return count")
		}},
		{"nullbnd-01", "boundary-as-numeric", BucketLatent, func(s string) string {
			return rep(s, "        None => { count = count + 0 }\n    }\n    return count",
				"        None => { count = count + 1 }\n    }\n    return count")
		}},
		{"nullbnd-01", "missing-close-deadlock", BucketLatent, func(s string) string {
			return rep(s, "    ch.close()\n", "")
		}},
		// ---- nullbnd-02 (pollScore) ----
		{"nullbnd-02", "closed-conflated-with-empty", BucketLatent, func(s string) string {
			return rep(s, "        Closed => { total = total + 100 }\n    }\n    return total",
				"        Closed => { total = total + 0 }\n    }\n    return total")
		}},
		{"nullbnd-02", "one-value-queued", BucketLatent, func(s string) string {
			return rep(s, "    ch.send(7)\n", "")
		}},
		{"nullbnd-02", "missing-close", BucketLatent, func(s string) string {
			return rep(s, "    ch.close()\n", "")
		}},
		{"nullbnd-02", "only-received-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "        Empty => { total = total + 0 }\n        Closed => { total = total + 100 }\n    }\n    let r2",
				"        Closed => { total = total + 100 }\n    }\n    let r2")
		}},
		// ---- race-01 (sharedCount) ----
		{"race-01", "task-captures-var-e1603", BucketRejected, func(s string) string {
			s = rep(s, "    let m = conc.Mutex(0)\n    scope {",
				"    let m = conc.Mutex(0)\n    var i: Int64 = 0\n    var j: Int64 = 0\n    scope {")
			s = rep(s, "        let a = task effect io {\n            var i: Int64 = 0", "        let a = task effect io {")
			return rep(s, "        let b = task effect io {\n            var j: Int64 = 0", "        let b = task effect io {")
		}},
		{"race-01", "record-capture-e1602", BucketRejected, func(s string) string {
			s = rep(s, "pub type AppError = Failed(String)",
				"pub type AppError = Failed(String)\n\nrecord Acc { n: Int64 }")
			s = rep(s, "    let m = conc.Mutex(0)", "    let m = conc.Mutex(0)\n    let c = Acc { n: 0 }")
			return rep(s, "        let a = task effect io {\n            var i: Int64 = 0",
				"        let a = task effect io {\n            let _ = c.n\n            var i: Int64 = 0")
		}},
		{"race-01", "unawaited-handle-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = b.await()\n", "")
		}},
		// ---- race-02 (mergeScale) ----
		{"race-02", "task-captures-var-e1603", BucketRejected, func(s string) string {
			return rep(s, "    let b = base", "    var b: Int64 = base")
		}},
		{"race-02", "unawaited-handle-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = c.await()\n", "")
		}},
		{"race-02", "same-partial-twice", BucketLatent, func(s string) string {
			return rep(s, "ch.send(b + 2)", "ch.send(b + 1)")
		}},
		{"race-02", "producer-never-spawned-deadlock", BucketLatent, func(s string) string {
			return rep(s, `        let c = task effect io {
            ch.send(b + 2)
        }
        let _ = a.await()
        let _ = c.await()`, "        let _ = a.await()")
		}},
		// ---- resleak-01 (drainSum) ----
		{"resleak-01", "missing-close-deadlock", BucketLatent, func(s string) string {
			return rep(s, "    ch.close()\n", "")
		}},
		{"resleak-01", "only-some-arm-e0305", BucketRejected, func(s string) string {
			return rep(s, "        None => { total = total + 0 }\n    }\n    return total",
				"    }\n    return total")
		}},
		{"resleak-01", "send-after-close-panic", BucketLatent, func(s string) string {
			return rep(s, "    ch.close()", "    ch.close()\n    ch.send(40)")
		}},
		{"resleak-01", "boundary-as-value", BucketLatent, func(s string) string {
			return rep(s, "        None => { total = total + 0 }\n    }\n    return total",
				"        None => { total = total + 1 }\n    }\n    return total")
		}},
		// ---- resleak-02 (guarded) ----
		{"resleak-02", "forgotten-release", BucketLatent, func(s string) string {
			return rep(s, "            defer { m.set(7) }\n", "")
		}},
		{"resleak-02", "defer-misplaced-e0204", BucketRejected, func(s string) string {
			return rep(s, "            defer { m.set(7) }",
				"            if true {\n                defer { m.set(7) }\n            }")
		}},
		{"resleak-02", "unawaited-handle-e1607", BucketRejected, func(s string) string {
			return rep(s, "        let _ = t.await()\n", "")
		}},
	}

	// Every case must be caught: the bucket is asserted exact, and clean is
	// structurally impossible (want clean is never listed).
	seen := map[string]bool{}
	for _, c := range cases {
		c := c
		seen[c.task] = true
		t.Run(c.task+"/"+c.trap, func(t *testing.T) {
			if c.want == BucketClean {
				t.Fatal("calibration case wants clean — not a trap")
			}
			files := attemptFiles(t, c.task, func(src string) string {
				out := c.mut(src)
				if out == src {
					t.Fatalf("mutation %s/%s did not change the source", c.task, c.trap)
				}
				return out
			})
			rep := Evaluate([]*Batch{{
				Model: "calibration",
				Task:  c.task,
				Attempts: []Attempt{
					{Round: 1, Files: files},
				},
			}})
			got := rep.Tasks[0].Attempts[0].Bucket
			if got != c.want {
				t.Fatalf("bucket %q, want %q", got, c.want)
			}
		})
	}
	// Coverage face: every seed task carries at least one calibrated trap.
	for _, task := range Tasks() {
		if !seen[task.ID] {
			t.Errorf("task %s has no calibrated trap case", task.ID)
		}
	}
}

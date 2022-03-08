// Package stack implements a very minimal version of the the stack
// package from "gopkg.in/inconshreveable/log15.v2/stack"
package stack

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// Call records a single function invocation from a goroutine stack. It is a
// wrapper for the program counter values returned by runtime.Caller and
// runtime.Callers and consumed by runtime.FuncForPC.
type Call uintptr

// Trace records a sequence of function invocations from a goroutine stack.
type Trace []Call

var pcStackPool = sync.Pool{
	New: func() interface{} { return make([]uintptr, 1000) },
}

func poolBuf() []uintptr {
	return pcStackPool.Get().([]uintptr)
}
func putPoolBuf(p []uintptr) {
	pcStackPool.Put(p)
}

// Callers return the list of callers from the current stack.
func Callers() Trace {
	pcs := poolBuf()
	pcs = pcs[:cap(pcs)]
	n := runtime.Callers(2, pcs)
	cs := make([]Call, n)
	for i, pc := range pcs[:n] {
		cs[i] = Call(pc)
	}
	putPoolBuf(pcs)
	return cs
}

// FunctionName provides the function name associated with the call
// point. It includes the module name as well.
func (pc Call) FunctionName() string {
	pcFix := uintptr(pc) - 1
	fn := runtime.FuncForPC(pcFix)
	if fn == nil {
		return "(nofunc)"
	}

	name := fn.Name()
	return name
}

// SourceFile returns the source file and optionally line number of
// the call point. The source file is relative to the import point
// (and includes it).
func (pc Call) SourceFile(withLine bool) string {
	pcFix := uintptr(pc) - 1
	fn := runtime.FuncForPC(pcFix)
	if fn == nil {
		return "(nofunc)"
	}

	const sep = "/"
	file, line := fn.FileLine(pcFix)
	impCnt := strings.Count(fn.Name(), sep) + 1
	pathCnt := strings.Count(file, sep)
	for pathCnt > impCnt {
		i := strings.Index(file, sep)
		if i == -1 {
			break
		}
		file = file[i+len(sep):]
		pathCnt--
	}
	if withLine {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return file
}

var (
	ownPackageCall    = Callers()[0]
	ownPackageName    = strings.SplitN(ownPackageCall.FunctionName(), ".", 2)[0] // akvorado/reporter/stack
	parentPackageName = ownPackageName[0:strings.LastIndex(ownPackageName, "/")] // akvorado/reporter

	// ModuleName is the name of the current module. This can be used to prefix stuff.
	ModuleName = parentPackageName[0:strings.LastIndex(parentPackageName, "/")] // akvorado
)

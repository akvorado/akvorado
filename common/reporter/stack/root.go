// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

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
	New: func() any {
		pcs := make([]uintptr, 32)
		return &pcs
	},
}

// Callers return the list of callers from the current stack.
func Callers() Trace {
	ptr := pcStackPool.Get().(*[]uintptr)
	pcs := *ptr
	n := runtime.Callers(2, pcs)
	cs := make([]Call, n)
	for i, pc := range pcs[:n] {
		cs[i] = Call(pc)
	}
	pcStackPool.Put(ptr)
	return cs
}

// CallInfo contains lazily-evaluated information from a Call.
// It performs a single runtime.FuncForPC lookup and caches values as they're accessed.
type CallInfo struct {
	pc           uintptr
	fn           *runtime.Func
	functionName string
	fileName     string
	sourceFile   string
	line         int
	initialized  bool
}

// Info creates a CallInfo that lazily extracts information from the call point.
// This is more efficient than calling FunctionName(), FileName(), and SourceFile() separately,
// and only computes values that are actually accessed.
func (pc Call) Info() *CallInfo {
	return &CallInfo{
		pc: uintptr(pc) - 1,
	}
}

func (ci *CallInfo) ensureInit() {
	if ci.initialized {
		return
	}
	ci.initialized = true
	ci.fn = runtime.FuncForPC(ci.pc)
	if ci.fn == nil {
		ci.functionName = "(nofunc)"
		ci.fileName = "(nofile)"
		ci.sourceFile = "(nosource)"
		return
	}
}

// FunctionName returns the function name.
func (ci *CallInfo) FunctionName() string {
	ci.ensureInit()
	if ci.functionName == "" {
		ci.functionName = ci.fn.Name()
	}
	return ci.functionName
}

// FileName returns the file name.
func (ci *CallInfo) FileName() string {
	ci.ensureInit()
	if ci.fileName == "" {
		ci.fileName, ci.line = ci.fn.FileLine(ci.pc)
	}
	return ci.fileName
}

// SourceFile returns the source file with line number.
func (ci *CallInfo) SourceFile() string {
	ci.ensureInit()
	if ci.sourceFile != "" {
		return ci.sourceFile
	}

	const sep = "/"
	sourceFile := ci.FileName()
	functionName := ci.FunctionName()
	impCnt := strings.Count(functionName, sep)
	pathCnt := strings.Count(sourceFile, sep)
	for pathCnt > impCnt {
		i := strings.Index(sourceFile, sep)
		if i == -1 {
			break
		}
		sourceFile = sourceFile[i+len(sep):]
		pathCnt--
	}
	i := strings.Index(functionName, ".")
	if i == -1 {
		ci.sourceFile = "(nosource)"
	} else {
		moduleName := functionName[:i]
		i = strings.Index(moduleName, "/")
		if i != -1 {
			moduleName = moduleName[:i]
		}
		ci.sourceFile = fmt.Sprintf("%s/%s:%d", moduleName, sourceFile, ci.line)
	}
	return ci.sourceFile
}

var (
	ownPackageCall    = Callers()[0]
	ownPackageName    = strings.SplitN(ownPackageCall.Info().FunctionName(), ".", 2)[0] // akvorado/common/reporter/stack
	parentPackageName = ownPackageName[0:strings.LastIndex(ownPackageName, "/")]        // akvorado/common/reporter

	// ModuleName is the name of the current module. This can be used to prefix stuff.
	ModuleName = strings.TrimSuffix(parentPackageName[0:strings.LastIndex(parentPackageName, "/")], "/common") // akvorado
)

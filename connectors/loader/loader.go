package main

import (
	"io/ioutil"
	"os"
	"fmt"
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/parser"
	"github.com/pkg/errors"

	"stackbrew.io/loader/ui"
)

func main() {
	// Load connectors
		// Q. connector list in commandline args?

	// Read cue input from stdin
	inputCue, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	f, err := parser.ParseFile("stdin", string(inputCue))
	if err != nil {
		panic(err)
	}
	r := new(cue.Runtime)
	i, err := r.CompileFile(f)
	if err != nil {
		panic(err)
	}

	// walkTasks(i.Value())

	fmt.Printf("tasks = %v\n", vHasTasks(i.Value(), nil))
	fmt.Printf("refs = %v\n", vHasRef(i.Value()))

	// Match contents of input with connector(s)
		// Q. match algorithm?
		// Q. support more than one connector?
		// Q. can connectors be nested?

	// Write cue output to stdout
}

// NEW CUE UTILITIES


type Task struct {
	Value cue.Value
	Backend string
}

func vLookupTask(v cue.Value) (t *Task, err error) {
	var (
		attr cue.Attribute
		backend string
	)
	attr = v.Attribute("task")
	if attr.Err() != nil {
		err = attr.Err()
		return
	}
	backend, err = attr.String(0)
	if err != nil {
		err = errors.Wrap(err, "invalid @task attribute")
		return
	}
	switch backend {
		case "exec":
			t = &Task{
				Backend: backend,
				Value: v,
			}
			ui.Info("task detected, backend=%s: %v", backend, t.Value)
		default:
			err = fmt.Errorf("unsupported task backend: %s", backend)
			ui.Error(err.Error())
	}
	return

}

// FIXME: return all tasks instead of returning just true/false
func vHasTasks(v cue.Value, path []string) (result bool) {
	ui.Info("%v\n\t%v", path, v)
	defer func() { ui.Info(" %v -> %v", path, result) }()
	// We are looking for a @task attribute anywhere within this value.
	// Does v have a @task attribute?
	_, err := vLookupTask(v)
	if err == nil {
		ui.Info("TASK DETECTED: %v", path)
		return true
	}

	// refI, refP := v.Reference()
	// if len(refP) > 0 {
	// 	info, err := refI.LookupField(refP...)
	// 	if err != nil {
	// 		ui.Info("error looking up reference %v: %s", refP, err)
	// 		return false
	// 	}
	// 	ui.Info("%v following reference to %v", path, refP)
	// 	if info.IsDefinition {
	// 		return vHasTasks(refI.LookupDef(refP...), refP)
	// 	}
	// 	return vHasTasks(refI.Lookup(refP...), refP)
	// }

	// If v is struct or list, recursively inspect its contents
	ui.Info("%v kind = %v", path, v.Kind())
	switch v.Kind() {
		case cue.StructKind:
			// Only iterate over "regular" fields (not hidden, eg. definitions)
			for it, _ := v.Fields(); it.Next(); {
				if vHasTasks(it.Value(), append(path, it.Label())) {
					return true
				}
			}
		case cue.ListKind:
			for it, _ := v.List(); it.Next(); {
				if vHasTasks(it.Value(), append(path, it.Label())) {
					return true
				}
			}
	}
	// If v is an expression, recursively inspect its component parts
	exprOp, exprArgs:= v.Expr()
	if exprOp != cue.NoOp && exprOp != cue.SelectorOp {
		for argIdx, arg := range(exprArgs) {
			// fakeLabel is used for human-friendly path display, only
			ui.Info("[%v] Following expression '%V'", path, exprOp)
			if vHasTasks(arg, append(path, fmt.Sprintf("%s/%d", exprOp.String(), argIdx))) {
				return true
			}
		}
	}
	return false

}

func walkTasks_naive(root cue.Value) {
	cueWalk(root, func(v cue.Value, path []string) (next bool) {
		ui.Info("WALK %v", path)
		next = true
		attr := v.Attribute("task")
		if err := attr.Err(); err != nil {
			return
		}
		backend, err := attr.String(0)
		if err != nil {
			ui.Error("invalid task attribute: %s", err)
			return
		}
		switch backend {
			case "exec":
				ui.Info("exec task detected  %v", path)
			default:
				ui.Error("unsupported task backend \"%s\": %v", backend, path)
		}
		return
	}, nil, nil)
}


// CUE UTILITIES COPIED FROM 54-qd

func walkDefinitions(root cue.Value) {
	cueWalk(root, func(v cue.Value, path []string) bool {
		for it, _ := v.Fields(cue.Definitions(true)); it.Next(); {
			if !it.IsDefinition() {
				continue
			}
			child := it.Value()
			ui.Info("\tDEFINITION: %v [concrete: %v]", append(path, it.Label()), child.IsConcrete())
		}
		return true
	}, nil, nil)
}

// cueWalk descends into all values of f
// Modified from https://cue.googlesource.com/cue/+/master/cue/types.go#1773
func cueWalk(v cue.Value, before func(cue.Value, []string) bool, after func(cue.Value, []string), path []string, opts ...cue.Option) {
	switch v.Kind() {
	case cue.StructKind:
		if before != nil && !before(v, path) {
			return
		}
		for it, _ := v.Fields(opts...); it.Next(); {
			cueWalk(it.Value(), before, after, append(path, it.Label()), opts...)
		}
	case cue.ListKind:
		if before != nil && !before(v, path) {
			return
		}
		list, _ := v.List()
		for i := 0; list.Next(); i++ {
			cueWalk(list.Value(), before, after, append(path, fmt.Sprintf("%d", i)), opts...)
		}
	default:
		if before != nil {
			before(v, path)
		}
	}
	if after != nil {
		after(v, path)
	}
}

func vHasRef(v cue.Value) (result bool) {
	// ui.Info("[vHasRef] %v", v)
	// defer func() { ui.Info("[vHasRef] -> %v", result) }()
	// Check for direct reference
	_, refP := v.Reference()
	if len(refP) > 0 {
		return true
	}
	// If v is strucr or list, recursively inspect its contents
	switch v.Kind() {
		case cue.StructKind:
			for it, _ := v.Fields(cue.All()); it.Next(); {
				if vHasRef(it.Value()) {
					return true
				}
			}
		case cue.ListKind:
			for it, _ := v.List(); it.Next(); {
				if vHasRef(it.Value()) {
					return true
				}
			}
	}
	// If v is an expression, recursively inspect its component parts
	exprOp, exprArgs:= v.Expr()
	if exprOp != cue.NoOp {
		for _, arg := range(exprArgs) {
			if vHasRef(arg) {
				return true
			}
		}
	}
	return false
}

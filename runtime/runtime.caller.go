package runtime

import (
	runt "runtime"
	"strings"
	"sync"
)

var (
	fnNamesMu sync.RWMutex
	fnNames   = make(map[uintptr]string)
	receivers = strings.NewReplacer("(", "", ")", "", "*", "")
)

func (rt *Runtime) getLogFnName(skip int) string {
	var pcs [1]uintptr
	if runt.Callers(skip+1, pcs[:]) == 0 {
		return "Unknown"
	}
	pc := pcs[0]

	fnNamesMu.RLock()
	name, ok := fnNames[pc]
	fnNamesMu.RUnlock()
	if ok {
		return name
	}

	frame, _ := runt.CallersFrames([]uintptr{pc}).Next()
	name = shortFnName(frame.Function)

	fnNamesMu.Lock()
	fnNames[pc] = name
	fnNamesMu.Unlock()

	return name
}

func shortFnName(full string) string {
	if full == "" {
		return "Unknown"
	}

	name := trimClosures(trimTypeArgs(full))
	idx := strings.LastIndex(name, ".")
	if idx == -1 {
		return name
	}

	pkg := name[:idx]
	mtd := name[idx+1:]
	if slash := strings.LastIndex(pkg, "/"); slash != -1 {
		pkg = pkg[slash+1:]
	}
	if dot := strings.LastIndex(pkg, "."); dot != -1 {
		pkg = pkg[dot+1:]
	}

	return receivers.Replace(pkg) + "." + mtd
}

func trimTypeArgs(name string) string {
	open := strings.Index(name, "[")
	if open == -1 {
		return name
	}

	close := strings.LastIndex(name, "]")
	if close < open {
		return name[:open]
	}

	return name[:open] + name[close+1:]
}

func trimClosures(name string) string {
	for {
		trimmed, ok := trimNumbered(name, ".func")
		if !ok {
			trimmed, ok = trimNumbered(name, "-range")
		}
		if !ok {
			return name
		}

		name = trimmed
	}
}

func trimNumbered(name string, marker string) (string, bool) {
	idx := strings.LastIndex(name, marker)
	if idx == -1 {
		return name, false
	}

	digits := name[idx+len(marker):]
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return name, false
	}

	return name[:idx], true
}

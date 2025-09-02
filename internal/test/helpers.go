package test

import (
	"path/filepath"
	"runtime"
)

func ProjectRoot() string {
	_, b, _, _ := runtime.Caller(0)
	// Root folder of this project is 3 levels up from this file
	return filepath.Join(filepath.Dir(b), "../..")
}

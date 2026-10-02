package main

import (
	"testing"

	v8 "github.com/ad3n/v8go"
)

func TestIssue91Crash(t *testing.T) {
	iso := v8.NewIsolate()
	ctx1 := v8.NewContext(iso)
	ctx1.RunScript("const multiply = (a, b) => a * b", "math.js")

	ctx2 := v8.NewContext(iso)
	if _, err := ctx2.RunScript("multiply(3, 4)", "main.js"); err != nil {

		t.Log(err)
	}
}

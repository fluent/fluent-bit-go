package fluentbit

/*
#include <stdlib.h>
*/
import "C"
import "github.com/fluent/fluent-bit-go/flbout"

// export Random
func Random() int {
	return int(C.random())
}

// export My_test
func My_test() *flbout.FLBPlugin {
	p := flbout.CreatePlugin("gstdout", "GO!")
	return p
}

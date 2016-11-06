package main

/*
#include <stdlib.h>
*/
import "C"
import "github.com/fluent/fluent-bit-go/flbout"
import "fmt"

func Random() int {
	return int(C.random())
}

func my_test() *flbout.FLBPlugin {
	p := flbout.CreatePlugin("gstdout", "GO!")
	return p
}

func main() {
	p := my_test()
	fmt.Printf("Testing: %v\n", p)
	fmt.Printf("Random: %d\n", Random())
}

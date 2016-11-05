package main

import "github.com/fluent/fluent-bit-go/flbout"
import "fmt"

func my_test() *flbout.FLBPlugin {
	p := flbout.CreatePlugin("gstdout", "GO!")
	return p
}

func main() {
	p := my_test()
	fmt.Printf("Testing: %v", p)
}

package main

import (
	"fmt"

	"github.com/fluent/fluent-bit-go"
)

func main() {
	p := fluentbit.My_test()
	fmt.Printf("Testing: %v\n", p)
	fmt.Printf("Random: %d\n", fluentbit.Random())
}

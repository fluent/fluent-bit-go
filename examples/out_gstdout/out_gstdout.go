package main

import "github.com/fluent/fluent-bit-go/output"
import (
	"github.com/ugorji/go/codec"
	"fmt"
	"unsafe"
	"C"
	"reflect"
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "gstdout", "Stdout GO!")
}

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {

	// Example to retrieve an optional configuration parameter
	param := output.FLBPluginConfigKey(ctx, "param")
	fmt.Printf("plugin parameter = '%s'\n", param)
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var count int
	var h codec.Handle = new(codec.MsgpackHandle)
	var b []byte
	var m interface{}
	var err error

	b = C.GoBytes(data, length)
	dec := codec.NewDecoderBytes(b, h)

	// Iterate the original MessagePack array
	count = 0
	for {
		// Decode the entry
		err = dec.Decode(&m)
		if err != nil {
			break
		}

		// Get a slice and their two entries: timestamp and map
		slice := reflect.ValueOf(m)
		timestamp := slice.Index(0)
		data := slice.Index(1)

		// Convert slice data to a real map and iterate
		map_data := data.Interface().(map[interface{}] interface{})
		fmt.Printf("[%d] %s: [%d, {", count, C.GoString(tag), timestamp)
		for k, v := range map_data {
			fmt.Printf("\"%s\": %v, ", k, v)
		}
		fmt.Printf("}\n")
		count++
	}

	// Return options:
	//
	// output.FLB_OK    = data have been processed.
	// output.FLB_ERROR = unrecoverable error, do not try this again.
	// output.FLB_RETRY = retry to flush later.
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}

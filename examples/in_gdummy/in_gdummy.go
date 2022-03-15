package main

import "C"
import (
	"fmt"
	"time"
	"reflect"
	"unsafe"

	"github.com/fluent/fluent-bit-go/input"
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return input.FLBPluginRegister(def, "gdummy", "dummy GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// plugin (context) pointer to fluentbit context (state/ c code)
func FLBPluginInit(plugin unsafe.Pointer) int {
	// Example to retrieve an optional configuration parameter
	param := input.FLBPluginConfigKey(plugin, "param")
	fmt.Printf("[flb-go] plugin parameter = '%s'\n", param)
	return input.FLB_OK
}

func MakePayload(packed []byte) (**C.void, int) {
	var payload **C.void

	length := len(packed)
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&payload))
	hdr.Data = uintptr(unsafe.Pointer(&packed))
	hdr.Len = length

	return payload, length
}

//export FLBPluginInputCallback
func FLBPluginInputCallback(data **C.void, size *C.size_t) int {
	now := time.Now()
	flb_time := input.FLBTime{now}
	message := map[string]string{"message": "dummy"}

	entry := []interface{}{flb_time, message}

	enc := input.NewEncoder()
	packed, err := enc.Encode(entry)
	if err != nil {
		fmt.Println("Can't convert to msgpack:", message, err)
		return input.FLB_ERROR
	}

	payload, length := MakePayload(packed)

	*data = *payload
	*size = C.size_t(length)
	// For emitting interval adjustment.
	time.Sleep(1000 * time.Millisecond)

	return input.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return input.FLB_OK
}

func main() {
}

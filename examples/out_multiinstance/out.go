package main

import (
	"C"
	"log"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "multiinstance", "Testing multiple instances.")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	id := output.FLBPluginConfigKey(plugin, "id")
	log.Printf("[multiinstance] id = %q", id)
	// Set the context to point to any Go variable
	output.FLBPluginSetContext(plugin, unsafe.Pointer(&id))
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	log.Print("Flush called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Cast context back into the original type for the Go variable
	id := (*string)(ctx)
	log.Printf("Flush called for id: %s", *id)

	dec := output.NewDecoder(data, int(length))

	for {
		ret, _, _ := output.GetRecord(dec)
		if ret != 0 {
			break
		}
	}

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}

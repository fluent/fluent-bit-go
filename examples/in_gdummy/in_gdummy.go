package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"time"
	"reflect"
	"sync"
	"unsafe"

	"github.com/fluent/fluent-bit-go/input"
)

type Slice struct {
	Data []byte
	data *c_slice_t
}

type c_slice_t struct {
	p unsafe.Pointer
	n int
}

var barrior sync.Mutex

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

func alloc(size int) unsafe.Pointer {
	return C.calloc(C.size_t(size), 1)
}

func makeSlice(p unsafe.Pointer, n int) *Slice {
	data := &c_slice_t{p: p, n: n}

	s := &Slice{data: data}
	h := (*reflect.SliceHeader)(unsafe.Pointer(&s.Data))
	h.Data = uintptr(p)
	h.Len = n
	h.Cap = n

	return s
}

//export FLBPluginInputCallback
func FLBPluginInputCallback(data *unsafe.Pointer, size *C.size_t) int {
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

	length := len(packed)
	p := alloc(length)
	s := makeSlice(p, length)
	copy(s.Data, packed)
	*data = unsafe.Pointer(&s.Data[0])
	*size = C.size_t(length)
	// For emitting interval adjustment.
	time.Sleep(1000 * time.Millisecond)

	return input.FLB_OK
}

//export FLBPluginInputCleanupCallback
func FLBPluginInputCleanupCallback(data unsafe.Pointer) int {
	barrior.Lock()
	C.free(unsafe.Pointer(data))
	barrior.Unlock()

	return input.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return input.FLB_OK
}

func main() {
}

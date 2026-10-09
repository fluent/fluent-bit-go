package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/fluent/fluent-bit-go/input"
)

// cmetrics msgpack schema constants (see cmetrics' cmt_encode_msgpack.h and cmetrics.h).
// They must be encoded as unsigned integers, so they are typed as uint64.
const (
	cmtMsgpackVersion uint64 = 2
	cmtCounter        uint64 = 0
	cmtGauge          uint64 = 1
)

var collections float64

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return input.FLBPluginRegisterWithOptions(def,
		input.WithName("gmetrics"),
		input.WithDescription("metrics GO!"),
		input.WithEventType(input.FLB_INPUT_METRICS))
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	return input.FLB_OK
}

// metric builds one cmetrics metric family: its metadata plus its samples.
// labelKeys are the label names shared by every sample of the family.
func metric(metricType uint64, name, desc string, labelKeys []string, samples ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"meta": map[string]interface{}{
			"ver":  cmtMsgpackVersion,
			"type": metricType,
			"opts": map[string]interface{}{
				"ns":   "go",
				"ss":   "gmetrics",
				"name": name,
				"desc": desc,
			},
			"labels": labelKeys,
		},
		"values": samples,
	}
}

// sample builds a single data point. The timestamp is in nanoseconds and must be
// an unsigned integer; the value must be a float64. labelValues follow the order
// of the family labelKeys.
func sample(ts uint64, value float64, labelValues ...string) map[string]interface{} {
	if labelValues == nil {
		// A nil slice is encoded as msgpack nil, but cmetrics expects an array.
		labelValues = []string{}
	}
	return map[string]interface{}{
		"ts":     ts,
		"value":  value,
		"labels": labelValues,
	}
}

//export FLBPluginInputCallback
func FLBPluginInputCallback(data *unsafe.Pointer, size *C.size_t) int {
	ts := uint64(time.Now().UnixNano())
	collections++

	// A cmetrics context: {"meta": {...}, "metrics": [...]}.
	context := map[string]interface{}{
		"meta": map[string]interface{}{
			"cmetrics":   map[string]interface{}{},
			"external":   map[string]interface{}{},
			"processing": map[string]interface{}{"static_labels": []interface{}{}},
		},
		"metrics": []interface{}{
			metric(cmtCounter, "collections_total", "Number of collections.", []string{"plugin"},
				sample(ts, collections, "gmetrics")),
			metric(cmtGauge, "goroutines", "Number of goroutines.", []string{},
				sample(ts, float64(runtime.NumGoroutine()))),
		},
	}

	enc := input.NewEncoder()
	packed, err := enc.Encode(context)
	if err != nil {
		fmt.Println("Can't convert to msgpack:", context, err)
		return input.FLB_ERROR
	}

	length := len(packed)
	*data = C.CBytes(packed)
	*size = C.size_t(length)
	// For emitting interval adjustment.
	time.Sleep(1000 * time.Millisecond)

	return input.FLB_OK
}

//export FLBPluginInputCleanupCallback
func FLBPluginInputCleanupCallback(data unsafe.Pointer) int {
	return input.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return input.FLB_OK
}

func main() {
}

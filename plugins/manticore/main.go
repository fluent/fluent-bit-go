package main

/*
#include <stdint.h>
*/
import "C"

import (
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "manticore", "Manticore native bulk output")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	context, err := newPluginContext(plugin)
	if err != nil {
		logError("initialization failed: %v", err)
		return output.FLB_ERROR
	}
	output.FLBPluginSetContext(plugin, context)
	logInfo(
		"initialized mode=%s endpoint=%s table=%q request_max_bytes=%d publish_source_bytes=%d request_timeout=%s",
		context.config.modeName(), context.config.endpointForLog(), context.config.Table,
		context.config.RequestMaxBytes, context.config.PublishSourceBytes,
		context.config.RequestTimeout,
	)
	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	context, ok := output.FLBPluginGetContext(ctx).(*pluginContext)
	if !ok || context == nil {
		logError("flush called without an initialized context")
		return output.FLB_ERROR
	}
	return context.flush(data, int(length))
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	logError("context-free flush is unsupported")
	return output.FLB_ERROR
}

//export FLBPluginExitCtx
func FLBPluginExitCtx(ctx unsafe.Pointer) int {
	context, ok := output.FLBPluginGetContext(ctx).(*pluginContext)
	if !ok || context == nil {
		return output.FLB_OK
	}
	return context.close()
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

//export FLBPluginUnregister
func FLBPluginUnregister(def unsafe.Pointer) {
	output.FLBPluginUnregister(def)
}

func main() {}

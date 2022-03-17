# Example: in_gdummy

The following example code implements an input plugin that works with
separated input collecting threads that is introduced in Fluent Bit 1.9.
It describes how to share context from the
specified instance configuration to the input callback.

Every output plugin go through four callbacks associated to different phases:

| Plugin Phase           | Callback                        |
|------------------------|---------------------------------|
| Registration           | FLBPluginRegister()             |
| Initialization         | FLBPluginInit()                 |
| Input Callback         | FLBPluginInputCallback()        |
| Input Cleanup Callback | FLBPluginInputCleanupCallback() |
| Exit                   | FLBPluginExit()                 |

## Plugin Registration

When Fluent Bit loads a Golang input plugin, it looks up and loads the registration
callback that aims to populate the internal structure with plugin name and
description:

```go
//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return input.FLBPluginRegister(def, "gdummy", "dummy Go!")
}
```

## Plugin Initialization

Before the engine starts, it initialize all plugins that were requested to start.
Upon initialization a configuration context already exists,
so the plugin can ask for configuration parameters or do any other internal checks. E.g:

```go
//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	return input.FLB_OK
}
```

The function must return FLB\_OK when it initialized properly or FLB\_ERROR if something went wrong. If the plugin reports an error, the engine will _not_ load the instance.

## Input Callback

When Fluent Bit wants to collect logs from Golang input plugin, the input callback will be triggered.

The callback will send a raw buffer of msgpack data with it proper bytes length into Fluent Bit core.

```go
import "reflect" // Import reflect package.

func alloc(size int) unsafe.Pointer {
	return C.calloc(C.size_t(size), 1)
}

func makeSlice(p unsafe.Pointer, n int) *Slice {
	data := &c_slice_t{p, n}

	s := &Slice{data: data}
	h := (*reflect.SliceHeader)(unsafe.Pointer(&s.Data))
	h.Data = uintptr(p)
	h.Len = n
	h.Cap = n

	return s
}

//export FLBPluginInputCallback
func FLBPluginInputCallback(data **C.void, size *C.size_t) int {
	now := time.Now()
	// To handle nanosecond precision on Golang input plugin, you must wrap up time instances with input.FLBTime type.
	flb_time := input.FLBTime{now}
	message := map[string]string{"message": "dummy"}

	entry := []interface{}{flb_time, message}

	// Some encoding logs to msgpack payload stuffs.
	// It needs to Wait for some period on Golang input plugin side, until the new records are emitted.

	length := len(packed)
	p := alloc(length)
	s := makeSlice(p, length)
	copy(s.Data, packed)
	*data = unsafe.Pointer(&s.Data[0])
	*size = C.size_t(len(packed))
	return input.FLB_OK
}
```

### Input Cleanup Callback

For cleaning up to be allocated resources, this callback will be triggered after _Input Callback_.

This callback is mainly used for deallocating heap memories which are allocated by `C.malloc` or `C.calloc`.

```go
import "sync"

var barrior sync.Mutex

//export FLBPluginInputCleanupCallback
func FLBPluginInputCleanupCallback(data unsafe.Pointer) int {
	barrior.Lock() // Guarding for deallocating region is needed for safety.
	C.free(unsafe.Pointer(data))
	barrior.Unlock()

	return input.FLB_OK
}
```

#### Returning Status Values

> for more details about how to process the sending msgpack data into Fluent Bit core, please refer to the [in_gdummy.go](in_gdummy.go) file.

When done, there are three returning values available:

| Return value  | Description                                    |
|---------------|------------------------------------------------|
| FLB\_OK       | The data have been processed normally.         |
| FLB\_ERROR    | An internal error have ocurred, the plugin will not handle the set of records/data again. |
| FLB\_RETRY    | A recoverable error have ocurred, the engine can try to flush the records/data later.|


## Plugin Exit

When Fluent Bit will stop using the instance of the plugin, it will trigger the exit callback. e.g:

```go
//export FLBPluginExit
func FLBPluginExit() int {
	return input.FLB_OK
}
```

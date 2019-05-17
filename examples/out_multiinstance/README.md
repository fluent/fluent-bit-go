# Example: out_multiinstance

The following example code implements an output plugin that works with
multiple configured instances. It describes how to share context from the
specified instance configuration to the flush callback.

Every output plugin go through four callbacks associated to different phases:

| Plugin Phase        | Callback                   |
|---------------------|----------------------------|
| Registration        | FLBPluginRegister()        |
| Initialization      | FLBPluginInit()            |
| Runtime Flush       | FLBPluginFlushCtx()        |
| Exit                | FLBPluginExit()            |

## Plugin Registration

When Fluent Bit loads a Golang plugin, it looks up and loads the registration
callback that aims to populate the internal structure with plugin name and
description:

```go
//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "multiinstance", "Testing multiple instances")
}
```

This function is invoked at start time _before_ any configuration is done
inside the engine.

## Plugin Initialization

Before the engine starts, it initializes all plugins that were configured.
As part of the initialization, the plugin can obtain configuration parameters
and do any other internal checks. It can also set the context for this
instance in case params need to be retrieved during flush.
E.g:

```go
//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
	id := output.FLBPluginConfigKey(plugin, "id")
	log.Printf("[multiinstance] id = %q", id)
	// Set the context to point to any Go variable
	output.FLBPluginSetContext(plugin, unsafe.Pointer(&id))
	return output.FLB_OK
}
```

The function must return FLB\_OK when it initialized properly or FLB\_ERROR if
something went wrong. If the plugin reports an error, the engine will _not_
load the instance.

## Runtime Flush with Context

Upon flush time, when Fluent Bit wants to flush it's buffers, the runtime flush
callback will be triggered.

The callback will receive the configuration context, a raw buffer of msgpack
data, the proper bytes length and the associated tag.

```go
//export FLBPluginFlushCtx
func FLBPluginFlush(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {

    id := *(*string)(ctx)
	log.Printf("[multiinstance] Flush called for id: %s", *id)
    return output.FLB_OK
}
```

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
	return output.FLB_OK
}
```

## Playground

Build the docker image locally to see how it works.

```bash
$ cd $GOPATH/src/github.com/fluent/fluent-bit-go/examples/out_multiinstance
$ docker build . -t fluent-bit-multiinstance
$ docker run -it --rm fluent-bit-multiinstance
```

# Example: in_gmetrics

The following example code implements an input plugin that emits **metrics**
instead of logs. It produces a counter with a label and a gauge every second.

> Requires a Fluent Bit version that includes
> [fluent/fluent-bit#11915](https://github.com/fluent/fluent-bit/pull/11915).
> On earlier versions the event type is ignored: the payload is parsed as log
> records, so the metrics are silently dropped.

The plugin goes through the same callbacks as [in_gdummy](../in_gdummy); only
the registration and the payload returned by the input callback differ.

## Plugin Registration

Register the plugin with the `input.WithEventType` option. The input event types
are `input.FLB_INPUT_LOGS` (default), `input.FLB_INPUT_METRICS` and
`input.FLB_INPUT_TRACES`:

```go
//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return input.FLBPluginRegisterWithOptions(def,
		input.WithName("gmetrics"),
		input.WithDescription("metrics GO!"),
		input.WithEventType(input.FLB_INPUT_METRICS))
}
```

The input event type constants have different values from the output ones
(`output.FLB_OUTPUT_METRICS`, ...), so always use the ones from the `input`
package.

## Input Callback

With `FLB_INPUT_METRICS`, the buffer returned by `FLBPluginInputCallback` must
be a msgpack-encoded [cmetrics](https://github.com/fluent/cmetrics) context
(the format produced by `cmt_encode_msgpack_create()`), not
`[timestamp, record]` log entries. Fluent Bit decodes it with
`cmt_decode_msgpack_create()`; if decoding fails, the payload is discarded and
`[proxy] failed to decode metrics msgpack (error N)` is logged.

The context has the following shape:

```
{
  "meta":    {"cmetrics": {}, "external": {}, "processing": {"static_labels": []}},
  "metrics": [
    {
      "meta": {
        "ver":    2,                     // cmetrics msgpack schema version
        "type":   0,                     // 0 = counter, 1 = gauge, 4 = untyped
        "opts":   {"ns": "go", "ss": "gmetrics", "name": "collections_total", "desc": "..."},
        "labels": ["plugin"]             // label keys
      },
      "values": [
        {"ts": 1700000000000000000, "value": 1.0, "labels": ["gmetrics"]}  // label values
      ]
    }
  ]
}
```

The decoder is strict about msgpack types:

- `ver`, `type` and `ts` must be **unsigned** integers. Use `uint64` in Go: the
  encoder writes a Go `int` larger than 127 as a signed msgpack integer, which
  is rejected.
- `ts` is the sample timestamp in **nanoseconds** (`time.Now().UnixNano()`).
- `value` must be a msgpack **float64**: use `float64`, not `int` or `float32`.
- `opts.desc` is mandatory. The metric name is `ns_ss_name` (empty parts are
  skipped), so the counter in this example is printed as
  `go_gmetrics_collections_total`.
- Each sample's `labels` holds the label values, in the same order as the
  `labels` keys of the metric `meta`. For metrics without labels use an empty
  list: a `nil` Go slice is encoded as msgpack nil and is rejected.
- A map can have at most 10 entries.

## Running

```
$ make
$ fluent-bit -e ./in_gmetrics.so -i gmetrics -o stdout -m '*'
```

`out_stdout` prints the metrics in text format:

```
2026-10-09T02:24:37.334439211Z go_gmetrics_collections_total{plugin="gmetrics"} = 1
2026-10-09T02:24:37.334439211Z go_gmetrics_goroutines = 2
2026-10-09T02:24:38.334972045Z go_gmetrics_collections_total{plugin="gmetrics"} = 2
2026-10-09T02:24:38.334972045Z go_gmetrics_goroutines = 2
```

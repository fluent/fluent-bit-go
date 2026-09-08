# Manticore output plugin

This Go output plugin sends Fluent Bit records to Manticore's native `/bulk`
endpoint. Choose a delivery mode with `Chunked`:

- `Off` (the default) sends each callback as one request with a fixed
  `Content-Length`. You can enable gzip compression in this mode.
- `On` collects callbacks in a durable local spool for assisted import. Each
  publication uploads the spool as one uncompressed HTTP chunked request to
  `/bulk?bulk_import=<table>`.

Create the destination table before using the plugin. Each record needs a stable,
positive numeric document ID. The plugin uses the existing schema and the IDs you
provide; it does not infer a schema or generate IDs.

## Build and load

Build the shared library on Linux with Go and cgo. It must be compatible with the
libc in your Fluent Bit image:

~~~bash
go build -buildvcs=false -trimpath -buildmode=c-shared \
  -o out_manticore.so ./plugins/manticore
fluent-bit -e /path/to/out_manticore.so -c /path/to/fluent-bit.conf
~~~

The library used for validation was built for Linux amd64 with Go 1.27.1 and glibc
2.28. It requires only the `GLIBC_2.2.5` and `GLIBC_2.3.2` symbol versions and
processed records on Fluent Bit 1.4.0 and 5.1.1. Build a separate library for other
architectures or ABIs.

## Configuration

| Property | Default | Meaning |
| --- | --- | --- |
| `Endpoint` | `http://127.0.0.1:9308` | Absolute HTTP or HTTPS base URL, without a query, fragment or embedded credentials. |
| `Table` | none | Required. Name of an existing Manticore table. |
| `Id_Key` | `id` | Record key containing the stable positive numeric ID. The plugin removes this key from the indexed document. |
| `Action` | `insert` | Only `insert` is supported. |
| `Chunked` | `Off` | `On` selects assisted import; `Off` selects ordinary delivery. |
| `Compress` | none | Set to `gzip` to compress ordinary requests. Requires `Chunked Off`. |
| `Spool_Path` | none | Required with `Chunked On`. Path to the data spool, used exclusively by this output. Its journal is `<path>.commit`. |
| `Request_Max_Bytes` | `64KiB` | Maximum source bytes returned by one read of an assisted request body. Records and publications can span multiple reads. |
| `Publish_Source_Bytes` | `0` | Assisted publication target, measured in bytes of encoded, uncompressed NDJSON. With `0`, only the finalizer triggers new publications. |
| `Request_Timeout` | `0` | Timeout for the whole request. `0` disables it. Use an integer for seconds or a Go duration such as `30s` or `5m`. Connections and TLS handshakes still have a 10-second timeout. |
| `HTTP_User` | none | Optional user for HTTP Basic authentication. |
| `HTTP_Passwd` | empty | Password for HTTP Basic authentication. Requires `HTTP_User`. |

Write byte sizes as integers with optional binary suffixes: `B`, `K`/`KB`/`KiB`,
`M`/`MB`/`MiB`, `G`/`GB`/`GiB`, or `T`/`TB`/`TiB`.

`Request_Max_Bytes` controls the transfer buffer. It does not cap the total request,
record or publication size, change Manticore's `max_packet_size`, or set TCP packet
sizes. A single record can span several reads and HTTP data frames.

The plugin checks `Publish_Source_Bytes` after it has encoded a complete callback
and synced both its data and acceptance journal. It keeps that callback whole, even
if doing so takes the spool past the target. The overshoot is at most that
callback's encoded NDJSON size.

HTTPS uses the operating system's trust store. The plugin has no option to select a
custom CA or disable TLS verification. Set credentials through `HTTP_User` and
`HTTP_Passwd`; `Endpoint` must not contain them. Logs include status, table and
spool details, but omit raw HTTP response bodies.

## Assisted finite import

Use one output instance with one Fluent Bit worker. Put the spool on private,
persistent local storage with enough free space for the largest publication:

~~~ini
[OUTPUT]
    Name                  manticore
    Match                 app.records
    Endpoint              https://search.example:9308
    Table                 app_events
    Id_Key                id
    Action                insert
    Chunked               On
    Spool_Path            /var/lib/fluent-bit/manticore/app-events.ndjson
    Request_Max_Bytes     64KiB
    Publish_Source_Bytes  0
    Request_Timeout       5m
    HTTP_User             fluentbit
    HTTP_Passwd           secret
    Workers               1
~~~

With `Publish_Source_Bytes 0`, a successful callback means its records are safely
stored in the local spool. Fluent Bit's finalizer triggers the upload at shutdown.
To upload batches of about 1 GiB as callbacks arrive, use:

~~~ini
    Chunked               On
    Spool_Path            /var/lib/fluent-bit/manticore/app-events.ndjson
    Request_Max_Bytes     64KiB
    Publish_Source_Bytes  1GiB
    Workers               1
~~~

The finalizer uploads any remaining records at shutdown.

Each publication uses one uncompressed HTTP/1.1 chunked request with
`bulk_import=<table>`. The plugin finishes the request body, waits for a valid
Manticore response, then closes the connection. Each completed request is its own
publication. Manticore's `bulk_import=0` releases assisted mode and its reservation;
it does not commit a group of earlier uploads. The Go plugin follows the C plugin
here and does not send a separate `bulk_import=0` request.

### Spool ownership and recovery

Create a private spool directory owned by the Fluent Bit service account. The
plugin creates the data file and `<Spool_Path>.commit` journal with mode 0600. If
you restore these files, preserve the service account's ownership and mode 0600.
The plugin does not check or repair the permissions or ownership of existing files.

The plugin opens the data file without following symlinks and takes an exclusive,
nonblocking lock. Give each output its own spool path; outputs and processes must
not share one. The journal records the endpoint, table, ID key and action, along
with callback boundaries and hashes. These let the plugin discard a partial tail
that was written but never accepted.

An assisted callback returns `FLB_OK` only after the plugin has synced all of its
NDJSON and its journal entry. After Manticore acknowledges the upload, the plugin
clears the spool and syncs those changes before reusing it. On startup, it replays
any accepted data before taking new input. If Manticore published the data but its
response was lost, replay can create another physical publication. This is
at-least-once delivery, so record IDs must stay the same across retries.

If an upload triggered by the size target fails, the callback that reached the
target still returns `FLB_OK`: the plugin has already accepted its records. Before
accepting another callback, it retries a transient failure by sending the same
whole spool. A permanent rejection makes it return `FLB_ERROR` for the new
callback. It never appends new records while a publication is unresolved.

### Shutdown and delivery failures

At shutdown, the finalizer retries a pending transient upload once, finishes
clearing an acknowledged spool, or uploads the remaining records. It reports a known
permanent rejection without uploading again. If publication remains unresolved,
the plugin keeps the accepted spool for recovery on the next start and returns
`FLB_ERROR`. The error log includes the table, spool path, byte, callback and
record counts, and the failure reason.

Fluent Bit can still exit with status 0 after a finalizer failure. The external Go
proxy does not reliably propagate that failure to the process exit status, and
returning `FLB_RETRY` does not schedule another attempt during shutdown. Check the
final publication log and retained spool to confirm delivery. Draining Fluent
Bit's input alone is not enough; the plugin has no completion-status API or
wrapper.

On Fluent Bit 1.4.0, run assisted mode in a process with no other Go output
instances. That version's Go proxy stores one finalizer for the whole process and
may call only the last registered instance's finalizer. Validation on 1.4.0 used a
dedicated process with one output, including tests of startup replay.

## Ordinary fixed and gzip delivery

Ordinary mode sends each callback directly to `/bulk` as one request with a fixed
`Content-Length`. It uses neither a spool nor `bulk_import`. The plugin returns
`FLB_OK` after a valid Manticore response, `FLB_RETRY` for retryable failures, and
`FLB_ERROR` for permanent record or response failures.

To send uncompressed requests:

~~~ini
[OUTPUT]
    Name             manticore
    Match            app.records
    Endpoint         http://manticore:9308
    Table            app_events
    Id_Key           id
    Chunked          Off
    Request_Timeout  30s
~~~

To compress each request with gzip, add `Compress gzip`:

~~~ini
[OUTPUT]
    Name             manticore
    Match            app.records
    Endpoint         http://manticore:9308
    Table            app_events
    Id_Key           id
    Chunked          Off
    Compress         gzip
    Request_Timeout  30s
~~~

Each ordinary request contains a complete callback, regardless of
`Request_Max_Bytes`, `Publish_Source_Bytes` or `Spool_Path`. If a callback exceeds
the daemon's request or JSON limits, the plugin reports the failure. It does not
split the callback or switch delivery modes.

## Records and IDs

Given this input record:

~~~json
{"id":42,"message":"ready","labels":{"region":"eu"}}
~~~

with `Table app_events`, the bulk operation is:

~~~json
{"insert":{"table":"app_events","id":42,"doc":{"message":"ready","labels":{"region":"eu"}}}}
~~~

Use a positive integer or a decimal UTF-8 string for the ID, within the `uint64`
range. The plugin rejects missing, zero, negative and out-of-range IDs, as well as
floating-point and binary values.

Document fields can contain nested maps with string keys, arrays, UTF-8 text,
booleans, finite numbers and null. The plugin rejects binary data, invalid UTF-8,
non-finite numbers and unsupported types.

The plugin leaves duplicate handling to Manticore. It keeps no duplicate-ID cache
and does not deduplicate across callbacks. Assisted replay can send the same
stable ID again.

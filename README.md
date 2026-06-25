# Fluent Bit Go!

This repository contains Go packages that allows to create [Fluent Bit][fluent-bit] plugins. At the moment it only supports the creation of _Output_ and _Input_ plugins.

## Requirements

The code of this package for output plugin is intended to be used with [Fluent Bit v1.4][fluent-bit-1-4] branch or higher.

The code of this package for input plugin is intended to be used with [Fluent Bit v1.9][fluent-bit-1-9] branch or higher.

## Usage

Fluent Bit Go packages are exposed on this repository:

[github.com/fluent/fluent-bit-go][fluent-bit-go]

### Creating Output Plugin

When creating a Fluent Bit Output plugin, the _output_ package can be used as follows:

```go
import "github.com/fluent/fluent-bit-go/output"
```

For a more practical example please refer to the [out_multiinstance plugin](./examples/out_multiinstance) implementation

### Creating Input Plugin

When creating a Fluent Bit Input plugin, the _input_ package can be used as follows:

```go
import "github.com/fluent/fluent-bit-go/input"
```

#### Config key constraint

Some config keys are used/overwritten by Fluent Bit and can't be used by a custom plugin, they are:

| Property                     | Input | Output |
|------------------------------|-------|--------|
| `alias`                        |   X   |   X    |
| `host`                         |   X   |   X    |
| `ipv6`                         |   X   |   X    |
| `listen`                       |   X   |        |
| `log_level`                    |   X   |   X    |
| `log_suppress_interval`        |   X   |        |
| `match`                        |   X   |        |
| `match_regex`                  |   X   |        |
| `mem_buf_limit`                |   X   |   X    |
| `port`                         |   X   |   X    |
| `retry_limit`                  |   X   |        |
| `routable`                     |   X   |   X    |
| `storage.pause_on_chunks_overlimit` | X |   X    |
| `storage.total_limit_size`     |   X   |        |
| `storage.type`                 |   X   |   X    |
| `tag`                          |   X   |   X    |
| `threaded`                     |   X   |   X    |
| `tls`                          |   X   |   X    |
| `tls.ca_file`                  |   X   |   X    |
| `tls.ca_path`                  |   X   |   X    |
| `tls.crt_file`                 |   X   |   X    |
| `tls.debug`                    |   X   |   X    |
| `tls.key_file`                 |   X   |   X    |
| `tls.key_passwd`               |   X   |   X    |
| `tls.verify`                   |   X   |   X    |
| `tls.vhost`                    |   X   |   X    |
| `workers`                      |   X   |        |

This implies that if your plugin depends on property like `listen` you can use on output plugin but not on input plugin, `host` don't work on both, rather than this custom key like `address` work on both.

```ini
[OUTPUT]
    Name   my_output_plugin
    Listen something # work
    Host # don't work
    Address # work
[INPUT]
    Name   my_input_plugin
    Listen something #don't work
    Host localhost # don't work
    Address # work
```

## Contact

Feel free to join us on our Slack channel, Mailing List, IRC or Twitter:

 - Slack: http://slack.fluentd.org (#fluent-bit channel)
 - Mailing List: https://groups.google.com/forum/#!forum/fluent-bit
 - IRC: irc.freenode.net #fluent-bit
 - Twitter: http://twitter.com/fluentbit

## Authors

[Fluent Bit Go][fluent-bit] is made and sponsored by [Treasure Data][treasure-data] among
other [contributors][contributors].

[fluent-bit]: http://fluentbit.io/
[fluent-bit-1-4]: https://github.com/fluent/fluent-bit/tree/v1.4.0
[fluent-bit-1-9]: https://github.com/fluent/fluent-bit/tree/1.9
[multiinstance]: https://github.com/fluent/fluent-bit-go/tree/fc386d263885e50387dd0081a77adf4072e8e4b6/examples/out_multiinstance
[fluent-bit-go]: http://github.com/fluent/fluent-bit-go
[treasure-data]: http://treasuredata.com
[contributors]: https://github.com/fluent/fluent-bit-go/graphs/contributors

# Fluent Bit Go!

This repository contains Go packages that allows to create [Fluent Bit][fluent-bit] plugins. At the moment it only supports the creation of _Output_ plugins.

## Requirements

The code of this package is intended to be used with [Fluent Bit v1.1][fluent-bit-1-1] branch.

## Usage

Fluent Bit Go packages are exposed on this repository:

[github.com/fluent/fluent-bit-go][fluent-bit-go]

When creating a Fluent Bit Output plugin, the _output_ package can be used as follows:

```go
import "github.com/fluent/fluent-bit-go/output"
```

For a more practical example please refer to the [out\_multiinstance plugin][multiinstance] implementation located at:

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
[fluent-bit-1-1]: https://github.com/fluent/fluent-bit/tree/v1.1.0
[multiinstance]: https://github.com/fluent/fluent-bit-go/tree/fc386d263885e50387dd0081a77adf4072e8e4b6/examples/out_multiinstance
[fluent-bit-go]: http://github.com/fluent/fluent-bit-go
[treasure-data]: http://treasuredata.com
[contributors]: https://github.com/fluent/fluent-bit-go/graphs/contributors

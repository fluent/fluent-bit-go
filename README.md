# Fluent Bit Go!

This repository contains Go packages that allows to create [Fluent Bit](http://fluentbit.io) plugins. At the moment it only supports the creation of _Output_ plugins.

## Usage

Fluent Bit Go packages are exposed on this repository:

[github.com/fluent/fluent-bit-go](http://github.com/fluent/fluent-bit-go)

When creating a Fluent Bit Output plugin, the _output_ package can be used as follows:

```go
import "github.com/fluent/fluent-bit-go/output"
```

for a more practical example please refer to the _out\_gstdout_ plugin implementation located at:

https://github.com/fluent/fluent-bit-go/blob/master/examples/out_gstdout/out_gstdout.go

## Contact

Feel free to join us on our Slack channel, Mailing List, IRC or Twitter:

 - Slack: http://slack.fluentd.org (#fluent-bit channel)
 - Mailing List: https://groups.google.com/forum/#!forum/fluent-bit
 - IRC: irc.freenode.net #fluent-bit
 - Twitter: http://twitter.com/fluentbit

## Authors

[Fluent Bit Go](http://fluentbit.io) is made and sponsored by [Treasure Data](http://treasuredata.com) among
other [contributors](https://github.com/fluent/fluent-bit/graphs/contributors).

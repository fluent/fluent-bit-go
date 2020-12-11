# Example: out_gpulsar

The following example code implements a simple output plugin that forward the records to Pulsar.

## Pulsar Configuration File

Pulsar configuration should be placed at /fluent-bit/etc/pulsar.conf. Sample config is shown below.

```
[PULSAR]
    URL    pulsar://localhost:6650
    Topic  fluent-bit
```

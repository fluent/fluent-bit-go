package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/calyptia/plugin"
)

func init() {
	plugin.RegisterOutput("go-test-output-plugin", "Golang output plugin for testing", dummyPlugin{})
}

type dummyPlugin struct{}

func (plug dummyPlugin) Init(ctx context.Context, conf plugin.ConfigLoader) error {
	return nil
}

func (plug dummyPlugin) Flush(ctx context.Context, ch <-chan plugin.Message) error {
	f, err := os.Create("/fluent-bit/etc/output.txt")
	if err != nil {
		return fmt.Errorf("could not open output.txt: %w", err)
	}

	defer f.Close()

	for msg := range ch {
		_, err := fmt.Fprintf(f, "message=\"got record\" tag=%s time=%s record=%+v\n", msg.Tag(), msg.Time.Format(time.RFC3339), msg.Record)
		if err != nil {
			return fmt.Errorf("could not write to output.txt: %w", err)
		}
	}

	return nil
}

func main() {}

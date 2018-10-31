package main

import "github.com/fluent/fluent-bit-go/output"
import (
	"C"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"unsafe"

	"github.com/Terry-Mao/goconf"
	"github.com/apache/pulsar/pulsar-client-go/pulsar"
)

func getFileContentAsString(filePath string) (string, error) {
	//logger.Infof("get file content as lines: %v", filePath)
	result := ""
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		//logger.Errorf("read file: %v error: %v", filePath, err)
		return result, err
	}

	return string(b), nil
}

func isMap(x interface{}) bool {
	t := fmt.Sprintf("%T", x)
	return strings.HasPrefix(t, "map[")
}

func getValue(m *string, i map[interface{}]interface{}) {
	*m += fmt.Sprintf("{")
	for k, v := range i {
		if isMap(v) {
			*m += fmt.Sprintf("\"%s\":", k)
			getValue(m, v.(map[interface{}]interface{}))
		} else {
			*m += fmt.Sprintf("\"%s\":\"%s\", ", k, v)
		}
	}
	*m += fmt.Sprintf("\b\b}, ")
}

var (
	client   pulsar.Client
	producer pulsar.Producer
)

func initPulsar() {
	var err error
	var url string
	var topic string

	conf := goconf.New()
	if err = conf.Parse("/fluent-bit/etc/pulsar.conf"); err != nil {
	}
	confpulsar := conf.Get("PULSAR")
	if confpulsar == nil {
		url = "pulsar://localhost:6650"
		topic = "fluent-bit"
	}
	url, err = confpulsar.String("URL")
	if err != nil {
		url = "pulsar://localhost:6650"
	}
	topic, err = confpulsar.String("Topic")
	if err != nil {
		topic = "fluent-bit"
	}

	// Instantiate a Pulsar client
	client, err = pulsar.NewClient(pulsar.ClientOptions{
		URL: url,
	})

	if err != nil {
		fmt.Println("Instantiate Pulsar client failed:", err)
	}

	// Use the client to instantiate a producer
	producer, err = client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})

	if err != nil {
		fmt.Println("instantiate Pulsar producer failed:", err)
	}
}

func deinitPulsar() {
	if producer != nil {
		producer.Close()
	}

	if client != nil {
		client.Close()
	}
}

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	initPulsar()
	return output.FLBPluginRegister(ctx, "gpulsar", "Pulsar GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
func FLBPluginInit(ctx unsafe.Pointer) int {
	// Example to retrieve an optional configuration parameter
	param := output.FLBPluginConfigKey(ctx, "param")
	fmt.Printf("[flb-go] plugin parameter = '%s'\n", param)
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	var ret int
	var record map[interface{}]interface{}

	// Create Fluent Bit decoder
	dec := output.NewDecoder(data, int(length))

	ctx := context.Background()

	// Iterate Records
	for {
		// Extract Record
		ret, _, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		var m string
		m = ""

		// Print record keys and values
		getValue(&m, record)
		m = strings.TrimSuffix(m, ", ")

		if producer != nil {
			// Create a message
			msg := pulsar.ProducerMessage{
				Payload: []byte(m),
			}

			// Attempt to send the message
			if err := producer.Send(ctx, msg); err != nil {
				fmt.Println("Pulsar send failed:", err)
			}
		}
	}

	// Return options:
	//
	// output.FLB_OK    = data have been processed.
	// output.FLB_ERROR = unrecoverable error, do not try this again.
	// output.FLB_RETRY = retry to flush later.
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	deinitPulsar()
	return output.FLB_OK
}

func main() {
}

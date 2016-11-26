package main

import "github.com/fluent/fluent-bit-go/output"
import (
  "fmt"
  "unsafe"
  "C"
  "github.com/ugorji/go/codec"
  "github.com/Shopify/sarama"
  "encoding/json"
  "io" 
  "reflect"
)

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
    return output.FLBPluginRegister(ctx, "out_kafka", "out_kafka GO!")
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
  var h codec.Handle = new(codec.MsgpackHandle)
  var b []byte
  var m interface{}
  var err error
  var enc_data []byte

  b = C.GoBytes(data, length)
  dec := codec.NewDecoderBytes(b, h)

  // Iterate the original MessagePack array
  for {
    // Decode the msgpack data
    err = dec.Decode(&m)
    if err != nil {
      if err == io.EOF {
        break
      }
      fmt.Printf("Failed to decode msgpack data: %v", err)
      return output.FLB_ERROR
    }

    // Encode the data as json
    format := "json"

    if format == "json" {
      enc_data, err = encode_as_json(m)
    } else if format == "msgpack" {
      enc_data, err = encode_as_msgpack(m)
    } else if format == "string" {
      // enc_data, err == encode_as_string(m)
    }
    if err != nil {
      fmt.Printf("Failed to encode %s data: %v", format, err)
      return output.FLB_ERROR
    }

    // Send message to kafka
    brokerList := []string{"localhost:9092"}
    config := sarama.NewConfig()
    producer, err := sarama.NewSyncProducer(brokerList, config)

    if err != nil {
      fmt.Printf("Failed to start Sarama producer: %v", err)
      return output.FLB_ERROR
    }

    producer.SendMessage(&sarama.ProducerMessage {
      Topic: "test",
      Key:   nil,
      Value: sarama.ByteEncoder(enc_data),
    })

    producer.Close()
  }

  // Return options:
  //
  // output.FLB_OK    = data have been processed.
  // output.FLB_ERROR = unrecoverable error, do not try this again.
  // output.FLB_RETRY = retry to flush later.
  return output.FLB_OK
}

func encode_as_json(m interface {}) ([]byte, error) {
  slice := reflect.ValueOf(m)
  timestamp := slice.Index(0).Interface().(uint64)
  record := slice.Index(1).Interface().(map[interface{}] interface{})

  // convert from map[interface{}] interface{} to map[string] interface{}
  // as JSON encoder can't encode non-string keys
  record2 := make(map[string] interface{})
  for k, v := range record {
    record2[k.(string)] = v
  }

  // TODO
  // add timestamp to record2, Marshal record2, remove log from

  type Log struct {
    Time uint64
    Record map[string] interface{}
  }

  log := Log {
    Time: timestamp,
    Record: record2,
  }

  return json.Marshal(log)
}

func encode_as_msgpack(m interface {}) ([]byte, error) {
  var (
    mh codec.MsgpackHandle
    w io.Writer
    b []byte
  )

  enc := codec.NewEncoder(w, &mh)
  enc = codec.NewEncoderBytes(&b, &mh)
  err := enc.Encode(&m)
  return b, err
}

// func encode_as_string(m interface {}) ([]byte, error) {

// }

// export FLBPluginExit
func FLBPluginExit() int {
  return 0
}

func main() {
}
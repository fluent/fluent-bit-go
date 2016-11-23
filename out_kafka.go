package main

import "github.com/fluent/fluent-bit-go/output"
import "github.com/Shopify/sarama"
import (
  "fmt"
  "reflect"
  "unsafe"
  "C"
  // "os"
  // "flag"
  // "strings"
  "github.com/ugorji/go/codec"
  // // "time"
  // "encoding/json"
  "io"
)

//export FLBPluginInit
func FLBPluginInit(ctx unsafe.Pointer) int {
    return output.FLBPluginRegister(ctx, "out_kafka", "out_kafka GO!")
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
  var count int
  var h codec.Handle = new(codec.MsgpackHandle)
  var b []byte
  var m interface{}
  var err error

  b = C.GoBytes(data, length)
  dec := codec.NewDecoderBytes(b, h)

  // Iterate the original MessagePack array
  count = 0
  for {
    // Decode the entry
    err = dec.Decode(&m)
    if err != nil {
      break
    }

    // Get two main entries: timestamp and map
    slice := reflect.ValueOf(m)
    timestamp := slice.Index(0)
    data := slice.Index(1)
    brokerList := []string{"localhost:9092"}
    // var message string

    // Convert slice map to a real map and iterate
    map_data := data.Interface().(map[interface{}] interface{})


    fmt.Printf("timestamp=%d\n", timestamp)
    fmt.Printf("count: %d", count)
    // time := fmt.Sprintf("%d", timestamp)

    for k, v := range map_data {
      fmt.Printf("     key[%s] value[%v]\n", k, v)
      // message = fmt.Sprintf("%v", v)
    }

    // establish struct for logs
    type Log struct {
      Timestamp string
      Log string
    }


    // msgpack
    var (
      w io.Writer
      b []byte
      mh codec.MsgpackHandle
    )


    enc := codec.NewEncoder(w, &mh)
    enc = codec.NewEncoderBytes(&b, &mh)
    err = enc.Encode(&m)
    // fmt.Printf("MESSSSAGEPACK: %v", b)



    // JSON
    // log := Log{
    //   Timestamp: time,
    //   Log: message,
    // }
    // // convert struct to json
    // enc, err := json.Marshal(log)
    // if err == nil {
    //   fmt.Println("error with MARSHAL:", err)
    // }



    // send message to kafka
    var er error
    config := sarama.NewConfig()
    producer, er := sarama.NewSyncProducer(brokerList, config)
    if er != nil {
      fmt.Printf("Failed to start Sarama producer:", err)
    }

    producer.SendMessage(&sarama.ProducerMessage {
      Topic: "test",
      Key:   nil,
      Value: sarama.ByteEncoder(b),
      // Value: sarama.ByteEncoder(encoded_log_bytes),
      // Value: sarama.StringEncoder(time),
    })

    count++
    producer.Close()
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
  return 0
}

func main() {
}
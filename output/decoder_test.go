//  Fluent Bit Go!
//  ==============
//  Copyright (C) 2015-2017 Treasure Data Inc.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package output

import (
	"encoding/binary"
	"log"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"
)

const (
	testDataFile = "./testdata/data"
)

// dummyRecord should be byte Array, not Slice to be able to Cast c array.
var dummyRecord [29]byte = [29]byte{0x92, /* fix array 2 */
	0xd7, 0x00, 0x5e, 0xa9, 0x17, 0xe0, 0x00, 0x00, 0x00, 0x00, /* 2020/04/29 06:00:00*/
	0x82,                                           /* fix map 2*/
	0xa7, 0x63, 0x6f, 0x6e, 0x70, 0x61, 0x63, 0x74, /* fix str 7 "compact" */
	0xc3,                                     /* true */
	0xa6, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, /* fix str 6 "schema" */
	0x01, /* fix int 1 */
}

// dummyV2Record should be byte Array, not Slice to be able to Cast c array.
var dummyV2Record [39]byte = [39]byte{0xdd /* array 32 */, 0x00, 0x00, 0x00,
	0x02, /* count of array elements */
	0xdd /* array 32 */, 0x00, 0x00, 0x00,
	0x02,                                                             /* count of array elements */
	0xd7, 0x00, 0x64, 0xbe, 0x0e, 0xeb, 0x16, 0x36, 0xe1, 0x28, 0x80, /* 2023/07/24 14:40:59 */
	0x82,                                           /* fix map 2 */
	0xa7, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x63, 0x74, /* fix str 7 "compact" */
	0xc3,                                     /* true */
	0xa6, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, /* fix str 6 "schema" */
	0x01, /* fix int 1 */
}

func TestGetRecord(t *testing.T) {
	dec := NewDecoder(unsafe.Pointer(&dummyRecord), len(dummyRecord))
	if dec == nil {
		t.Fatal("dec is nil")
	}

	ret, timestamp, record := GetRecord(dec)
	if ret < 0 {
		t.Fatal("ret is negative")
	}

	// test timestamp
	ts, ok := timestamp.(FLBTime)
	if !ok {
		t.Fatalf("cast error. Type is %s", reflect.TypeOf(timestamp))
	}

	if ts.Unix() != int64(0x5ea917e0) {
		t.Errorf("ts.Unix() error. given %d", ts.Unix())
	}

	// test record
	v, ok := record["schema"].(int64)
	if !ok {
		t.Fatalf("cast error. Type is %s", reflect.TypeOf(record["schema"]))
	}
	if v != 1 {
		t.Errorf(`record["schema"] is not 1 %d`, v)
	}
}

func TestGetV2Record(t *testing.T) {

	anotherDummyV2RecordBytes, err := os.ReadFile(testDataFile)
	if err != nil {
		log.Fatal(err)
	}
	anotherDummyV2Record := (*[39]byte)(anotherDummyV2RecordBytes)

	t.Log(dummyV2Record)
	t.Log(*anotherDummyV2Record)

	dummyV2Records := [][39]byte{
		dummyV2Record,
		*anotherDummyV2Record,
	}

	for i, record := range dummyV2Records {

		dec := NewDecoder(unsafe.Pointer(&record), len(record))
		if dec == nil {
			t.Fatal("dec is nil, i", i)
		}

		ret, timestamp, record := GetRecord(dec)
		if ret < 0 {
			t.Fatalf("ret is negative: code %v", ret)
		}

		// test timestamp
		ts, ok := timestamp.(FLBTime)
		if !ok {
			t.Fatalf("cast error. Type is %s", reflect.TypeOf(timestamp))
		}

		// test
		if ts.Time != extractTimeStamp() {
			t.Fatalf("GetRecord timestamp extraction does not match manual extractTimeStamp")
		}

		if ts.Unix() != int64(0x64be0eeb) {
			t.Errorf("ts.Unix() error. given %d", ts.Unix())
		}

		// test record
		v, ok := record["schema"].(int64)
		if !ok {
			t.Fatalf("cast error. Type is %s", reflect.TypeOf(record["schema"]))
		}
		if v != 1 {
			t.Errorf(`record["schema"] is not 1 %d`, v)
		}
	}
}

// extractTimeStamp extracts the time out of the MsgPack data without using refection
func extractTimeStamp() (ts time.Time) {

	data := unsafe.Slice((*byte)(unsafe.Pointer(&dummyV2Record)), int(len(dummyV2Record)))

	// Manually find the time.
	//
	// The first headers of the data coming from Fluentbit looks like this
	//~/Downloads/msgpack-inspect ./data
	// ---
	// - format: "array32"
	//   header: "0xdd"
	//   length: 2
	//   children:
	//     - format: "array32"
	//       header: "0xdd"
	//       length: 2
	//       children:
	//         - format: "fixext8"
	//           header: "0xd7"
	//           exttype: 0
	//           length: 8
	//           data: "0x64a46baa019bfcc0"
	//         - format: "fixmap"
	//           header: "0x80"
	//           length: 0
	//           children: []
	// Array32 is 5 bytes long, and the fixext8 has x2 byte header, so that's where the time field starts
	// fixext8 has 8 bytes of data
	// Therefore, time bits start at: 5 + 5 + 2 = 12
	// Time bits are 8 long, so 12 + 8 = 20
	// See also: https://github.com/msgpack/msgpack/blob/master/spec.md
	//
	timeEightBytes := data[12:20] // extract bytes 12 through 20
	sec := binary.BigEndian.Uint32(timeEightBytes)
	usec := binary.BigEndian.Uint32(timeEightBytes[4:])
	ts = time.Unix(int64(sec), int64(usec))

	//log.Println("timestamp:", timestamp.Format(time.RFC850)

	return ts
}

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
	"reflect"
	"testing"
	"unsafe"
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

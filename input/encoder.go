//  Fluent Bit Go!
//  ==============
//  Copyright (C) 2022 The Fluent Bit Go Authors
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

package input

import (
	"C"
	"encoding/binary"
	"reflect"
	"time"

	"github.com/ugorji/go/codec"
)

type FLBEncoder struct {
	handle *codec.MsgpackHandle
	mpenc  *codec.Encoder
}

type FLBTime struct {
	time.Time
}

func (f FLBTime) WriteExt(i interface{}) []byte {
	bs := make([]byte, 8)

	tm := i.(*FLBTime)
	utc := tm.UTC()

	sec := utc.Unix()
	nsec := utc.Nanosecond()

	binary.BigEndian.PutUint32(bs, uint32(sec))
	binary.BigEndian.PutUint32(bs[4:], uint32(nsec))

	return bs
}

func (f FLBTime) ReadExt(i interface{}, b []byte) {
	panic("unsupported")
}

func NewEncoder() *FLBEncoder {
	enc := new(FLBEncoder)
	enc.handle = new(codec.MsgpackHandle)
	enc.handle.WriteExt = true
	// TODO: handle error.
	_ = enc.handle.SetBytesExt(reflect.TypeOf(FLBTime{}), 0, &FLBTime{})

	return enc
}

func (enc *FLBEncoder) Encode(val interface{}) (packed []byte, err error) {
	enc.mpenc = codec.NewEncoderBytes(&packed, enc.handle)
	err = enc.mpenc.Encode(val)
	return
}

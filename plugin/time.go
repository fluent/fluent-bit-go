package plugin

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ugorji/go/codec"
)

const fFimeSize = 8

var _ codec.BytesExt = (*bigEndianTime)(nil)

// bigEndianTime implements codec.BytesExt to handle custom (de)serialization of types to/from []byte.
// It is used by codecs (e.g. binc, msgpack, simple) which do custom serialization of the types.
type bigEndianTime time.Time

func (*bigEndianTime) WriteExt(v interface{}) []byte {
	ft, ok := v.(*bigEndianTime)
	if !ok {
		panic(fmt.Sprintf("unexpected big endian time type %T", v))
	}

	t := time.Time(*ft).UTC()
	sec := t.Unix()
	nsec := t.UnixNano()

	b := make([]byte, fFimeSize)
	binary.BigEndian.PutUint32(b, uint32(sec))
	binary.BigEndian.PutUint32(b[4:], uint32(nsec))

	return b
}

func (*bigEndianTime) ReadExt(dst interface{}, src []byte) {
	ft, ok := dst.(*bigEndianTime)
	if !ok {
		panic(fmt.Sprintf("unexpected big endian time type %T", dst))
	}

	if len(src) != fFimeSize {
		panic(fmt.Sprintf("unexpected big endian time length %d", len(src)))
	}

	sec := binary.BigEndian.Uint32(src)
	nsec := binary.BigEndian.Uint32(src[4:])

	t := time.Unix(int64(sec), int64(nsec)).UTC()
	*ft = bigEndianTime(t)
}

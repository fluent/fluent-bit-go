package main

import "C"

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/ugorji/go/codec"
)

type encodeStats struct {
	Records int64
	Bytes   int64
}

type countingWriter struct {
	writer io.Writer
	bytes  int64
	err    error
}

func (w *countingWriter) Write(data []byte) (int, error) {
	written, err := w.writer.Write(data)
	w.bytes += int64(written)
	if err != nil && w.err == nil {
		w.err = err
	}
	return written, err
}

func encodeCallback(data unsafe.Pointer, length int, config pluginConfig, writer io.Writer) (encodeStats, error) {
	if length == 0 {
		return encodeStats{}, nil
	}
	// Keep the decoder error and STR/BIN distinction, which the SDK wrapper loses.
	handle := &codec.MsgpackHandle{WriteExt: true}
	if err := handle.SetBytesExt(reflect.TypeOf(output.FLBTime{}), 0, &output.FLBTime{}); err != nil {
		return encodeStats{}, fmt.Errorf("could not initialize log decoder: %w", err)
	}
	decoder := codec.NewDecoderBytes(C.GoBytes(data, C.int(length)), handle)
	counting := &countingWriter{writer: writer}
	stats := encodeStats{}
	// This codec can return io.EOF inside a truncated record. Only the known
	// callback boundary is clean exhaustion; any attempted-record error rejects it.
	for decoder.NumBytesRead() < length {
		var value interface{}
		if err := decoder.Decode(&value); err != nil {
			return encodeStats{}, fmt.Errorf("could not decode Fluent Bit log record: %w", err)
		}
		event, ok := value.([]interface{})
		if !ok || len(event) != 2 {
			return encodeStats{}, fmt.Errorf("Fluent Bit log record must be [timestamp, map]")
		}
		switch timestamp := event[0].(type) {
		case output.FLBTime, uint64:
		case []interface{}: // Fluent Bit v2 [timestamp, metadata] header.
			if len(timestamp) < 2 {
				return encodeStats{}, fmt.Errorf("invalid Fluent Bit metadata header")
			}
		default:
			return encodeStats{}, fmt.Errorf("unsupported Fluent Bit timestamp type %T", timestamp)
		}
		record, ok := event[1].(map[interface{}]interface{})
		if !ok {
			return encodeStats{}, fmt.Errorf("Fluent Bit log record body must be a map")
		}
		if err := encodeRecord(record, config, counting); err != nil {
			if counting.err != nil {
				return encodeStats{}, fmt.Errorf("could not write native bulk record: %w", counting.err)
			}
			return encodeStats{}, err
		}
		stats.Records++
	}
	stats.Bytes = counting.bytes
	return stats, nil
}

func encodeRecord(record map[interface{}]interface{}, config pluginConfig, writer io.Writer) error {
	id, doc, err := splitRecord(record, config.IDKey)
	if err != nil {
		return err
	}
	envelope := struct {
		Insert struct {
			Table string                 `json:"table"`
			ID    uint64                 `json:"id"`
			Doc   map[string]interface{} `json:"doc"`
		} `json:"insert"`
	}{}
	envelope.Insert.Table = config.Table
	envelope.Insert.ID = id
	envelope.Insert.Doc = doc

	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(&envelope); err != nil {
		return fmt.Errorf("could not encode native bulk record: %w", err)
	}
	return nil
}

func splitRecord(record map[interface{}]interface{}, idKey string) (uint64, map[string]interface{}, error) {
	if record == nil {
		return 0, nil, fmt.Errorf("record must be a map")
	}
	doc := make(map[string]interface{}, len(record))
	var id uint64
	idFound := false
	for rawKey, rawValue := range record {
		key, err := normalizeKey(rawKey)
		if err != nil {
			return 0, nil, err
		}
		if key == idKey {
			if idFound {
				return 0, nil, fmt.Errorf("record key %q appears more than once", idKey)
			}
			id, err = parseDocumentID(rawValue)
			if err != nil {
				return 0, nil, fmt.Errorf("record key %q %w", idKey, err)
			}
			idFound = true
			continue
		}
		if _, exists := doc[key]; exists {
			return 0, nil, fmt.Errorf("record key %q appears more than once", key)
		}
		value, err := normalizeValue(rawValue)
		if err != nil {
			return 0, nil, fmt.Errorf("record key %q: %w", key, err)
		}
		doc[key] = value
	}
	if !idFound {
		return 0, nil, fmt.Errorf("record must contain a positive numeric ID in key %q", idKey)
	}
	return id, doc, nil
}

func parseDocumentID(value interface{}) (uint64, error) {
	var id uint64
	switch typed := value.(type) {
	case uint:
		id = uint64(typed)
	case uint8:
		id = uint64(typed)
	case uint16:
		id = uint64(typed)
	case uint32:
		id = uint64(typed)
	case uint64:
		id = typed
	case int:
		if typed <= 0 {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = uint64(typed)
	case int8:
		if typed <= 0 {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = uint64(typed)
	case int16:
		if typed <= 0 {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = uint64(typed)
	case int32:
		if typed <= 0 {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = uint64(typed)
	case int64:
		if typed <= 0 {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = uint64(typed)
	case string:
		parsed, err := strconv.ParseUint(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("must be a positive numeric ID")
		}
		id = parsed
	default:
		return 0, fmt.Errorf("must be a positive numeric ID")
	}
	if id == 0 {
		return 0, fmt.Errorf("must be a positive numeric ID")
	}
	return id, nil
}

func normalizeKey(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		if !utf8.ValidString(typed) {
			return "", fmt.Errorf("record contains a non-UTF-8 map key")
		}
		return typed, nil
	default:
		return "", fmt.Errorf("record map keys must be strings")
	}
}

func normalizeValue(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		if !utf8.ValidString(typed) {
			return nil, fmt.Errorf("non-UTF-8 strings are unsupported")
		}
		return typed, nil
	case bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return typed, nil
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return nil, fmt.Errorf("non-finite floating-point values are unsupported")
		}
		return typed, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, fmt.Errorf("non-finite floating-point values are unsupported")
		}
		return typed, nil
	case json.Number:
		parsed, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil || !json.Valid([]byte(typed.String())) || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, fmt.Errorf("invalid JSON number")
		}
		return typed, nil
	case []byte:
		return nil, fmt.Errorf("binary values are unsupported")
	case map[interface{}]interface{}:
		return normalizeMap(typed)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			if _, err := normalizeKey(key); err != nil {
				return nil, err
			}
			normalized, err := normalizeValue(child)
			if err != nil {
				return nil, fmt.Errorf("nested key %q: %w", key, err)
			}
			out[key] = normalized
		}
		return out, nil
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Array, reflect.Slice:
		out := make([]interface{}, reflected.Len())
		for index := 0; index < reflected.Len(); index++ {
			normalized, err := normalizeValue(reflected.Index(index).Interface())
			if err != nil {
				return nil, fmt.Errorf("array element %d: %w", index, err)
			}
			out[index] = normalized
		}
		return out, nil
	case reflect.Map:
		out := make(map[string]interface{}, reflected.Len())
		iterator := reflected.MapRange()
		for iterator.Next() {
			key, err := normalizeKey(iterator.Key().Interface())
			if err != nil {
				return nil, err
			}
			if _, exists := out[key]; exists {
				return nil, fmt.Errorf("nested key %q appears more than once", key)
			}
			normalized, err := normalizeValue(iterator.Value().Interface())
			if err != nil {
				return nil, fmt.Errorf("nested key %q: %w", key, err)
			}
			out[key] = normalized
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", value)
	}
}

func normalizeMap(value map[interface{}]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{}, len(value))
	for rawKey, child := range value {
		key, err := normalizeKey(rawKey)
		if err != nil {
			return nil, err
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("nested key %q appears more than once", key)
		}
		normalized, err := normalizeValue(child)
		if err != nil {
			return nil, fmt.Errorf("nested key %q: %w", key, err)
		}
		out[key] = normalized
	}
	return out, nil
}

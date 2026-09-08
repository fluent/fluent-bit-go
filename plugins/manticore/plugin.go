package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

type pluginContext struct {
	mu         sync.Mutex
	config     pluginConfig
	client     *bulkClient
	spool      *durableSpool
	pending    pendingPublication
	rawBody    bytes.Buffer
	gzipBody   bytes.Buffer
	gzipWriter *gzip.Writer
	closed     bool
}

type pendingPhase int

const (
	pendingNone pendingPhase = iota
	pendingUpload
	pendingRetirement
)

type pendingPublication struct {
	Phase pendingPhase
	Class failureClass
	Err   error
}

func newPluginContext(plugin unsafe.Pointer) (*pluginContext, error) {
	config, err := readConfig(plugin)
	if err != nil {
		return nil, err
	}
	context := &pluginContext{config: config, client: newBulkClient(config)}
	if config.Mode != modeAssisted {
		return context, nil
	}
	spool, err := openDurableSpool(config)
	if err != nil {
		context.client.close()
		return nil, err
	}
	context.spool = spool
	if spool.acceptedBytes == 0 {
		return context, nil
	}
	logInfo("replaying accepted spool table=%q spool=%q bytes=%d callbacks=%d records=%d",
		config.Table, config.SpoolPath, spool.acceptedBytes, spool.acceptedCallbacks, spool.acceptedRecords)
	result := context.client.sendAssisted(spool.reader(), spool.acceptedBytes)
	if !result.succeeded() {
		_ = spool.close()
		context.client.close()
		return nil, fmt.Errorf(
			"could not replay accepted spool %q (request_source_bytes=%d request_sha256=%s body_reads=%d): %w",
			config.SpoolPath, result.SourceBytes, result.SourceHash, result.BodyReads, result.Err,
		)
	}
	if failure := spool.retire(); failure != nil {
		_ = spool.close()
		context.client.close()
		return nil, fmt.Errorf("could not retire replayed spool %q: %w", config.SpoolPath, failure.Err)
	}
	logInfo("replayed and retired spool table=%q bytes=%d sha256=%s", config.Table, result.SourceBytes, result.SourceHash)
	return context, nil
}

func (p *pluginContext) flush(data unsafe.Pointer, length int) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return output.FLB_ERROR
	}
	if length == 0 {
		return output.FLB_OK
	}
	if p.config.Mode != modeAssisted {
		return p.flushOrdinary(data, length)
	}
	if p.pending.Phase != pendingNone {
		if failure := p.resolvePending(); failure != nil {
			logError("new callback not accepted while publication is pending table=%q spool=%q bytes=%d callbacks=%d records=%d: %v",
				p.config.Table, p.config.SpoolPath, p.spool.acceptedBytes,
				p.spool.acceptedCallbacks, p.spool.acceptedRecords, failure.Err)
			return failureCode(failure.Class)
		}
	}
	stats, failure := p.spool.appendCallback(data, length, p.config)
	if failure != nil {
		logError("could not accept callback table=%q spool=%q: %v", p.config.Table, p.config.SpoolPath, failure.Err)
		return failureCode(failure.Class)
	}
	if stats.Records > 0 {
		logInfo("accepted callback table=%q spool=%q callback_records=%d callback_bytes=%d spool_callbacks=%d spool_records=%d spool_bytes=%d",
			p.config.Table, p.config.SpoolPath, stats.Records, stats.Bytes,
			p.spool.acceptedCallbacks, p.spool.acceptedRecords, p.spool.acceptedBytes)
	}
	if stats.Records > 0 && p.config.PublishSourceBytes > 0 &&
		p.spool.acceptedBytes >= p.config.PublishSourceBytes {
		if failure := p.publishAccepted("size-triggered"); failure != nil {
			logError("size-triggered publication remains pending after accepted callback table=%q spool=%q bytes=%d callbacks=%d records=%d; callback ownership retained: %v",
				p.config.Table, p.config.SpoolPath, p.spool.acceptedBytes,
				p.spool.acceptedCallbacks, p.spool.acceptedRecords, failure.Err)
		}
	}
	return output.FLB_OK
}

func (p *pluginContext) flushOrdinary(data unsafe.Pointer, length int) int {
	p.rawBody.Reset()
	stats, err := encodeCallback(data, length, p.config, &p.rawBody)
	if err != nil {
		logError("could not encode ordinary callback table=%q: %v", p.config.Table, err)
		return output.FLB_ERROR
	}
	if stats.Records == 0 {
		return output.FLB_OK
	}
	encoded := p.rawBody.Bytes()
	encodedHash := sha256.Sum256(encoded)
	source := ordinaryRequestSource(encoded)
	sourceHash := sha256.Sum256(source)
	requestBody := source
	if p.config.CompressGZIP {
		p.gzipBody.Reset()
		if p.gzipWriter == nil {
			p.gzipWriter = gzip.NewWriter(&p.gzipBody)
		} else {
			p.gzipWriter.Reset(&p.gzipBody)
		}
		if _, err := p.gzipWriter.Write(source); err != nil {
			logError("could not compress ordinary callback table=%q: %v", p.config.Table, err)
			return output.FLB_RETRY
		}
		if err := p.gzipWriter.Close(); err != nil {
			logError("could not finish ordinary callback compression table=%q: %v", p.config.Table, err)
			return output.FLB_RETRY
		}
		requestBody = p.gzipBody.Bytes()
	}
	result := p.client.sendOrdinary(requestBody, p.config.CompressGZIP)
	if !result.succeeded() {
		logError("ordinary publication failed table=%q mode=%s records=%d encoded_bytes=%d encoded_sha256=%s source_bytes=%d source_sha256=%s request_bytes=%d: %v",
			p.config.Table, p.config.modeName(), stats.Records, len(encoded),
			hex.EncodeToString(encodedHash[:]), len(source), hex.EncodeToString(sourceHash[:]),
			len(requestBody), result.Err)
		return failureCode(result.Class)
	}
	logInfo("ordinary publication acknowledged table=%q mode=%s records=%d encoded_bytes=%d encoded_sha256=%s source_bytes=%d source_sha256=%s request_bytes=%d",
		p.config.Table, p.config.modeName(), stats.Records, len(encoded),
		hex.EncodeToString(encodedHash[:]), len(source), hex.EncodeToString(sourceHash[:]),
		len(requestBody))
	return output.FLB_OK
}

func ordinaryRequestSource(encoded []byte) []byte {
	// Manticore's current fixed-length gzip reader includes its appended C NUL
	// in the NDJSON stream. A terminal LF would expose that NUL as a second,
	// invalid line. Request EOF commits the final record, so omit only the last
	// delimiter in both ordinary variants and keep their decoded bodies equal.
	if len(encoded) > 0 && encoded[len(encoded)-1] == '\n' {
		return encoded[:len(encoded)-1]
	}
	return encoded
}

func (p *pluginContext) close() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return output.FLB_OK
	}
	p.closed = true
	resultCode := output.FLB_OK
	if p.config.Mode == modeAssisted {
		var failure *spoolFailure
		if p.pending.Phase != pendingNone {
			failure = p.resolvePending()
		} else if p.spool.acceptedBytes > 0 {
			failure = p.publishAccepted("final")
		}
		if failure != nil {
			logError("final publication unresolved table=%q spool=%q bytes=%d callbacks=%d records=%d; accepted data retained: %v",
				p.config.Table, p.config.SpoolPath, p.spool.acceptedBytes,
				p.spool.acceptedCallbacks, p.spool.acceptedRecords, failure.Err)
			resultCode = output.FLB_ERROR
		}
	}
	if p.spool != nil {
		if err := p.spool.close(); err != nil {
			logError("could not close spool table=%q spool=%q: %v", p.config.Table, p.config.SpoolPath, err)
			resultCode = output.FLB_ERROR
		}
	}
	p.client.close()
	return resultCode
}

func (p *pluginContext) publishAccepted(reason string) *spoolFailure {
	if p.spool.acceptedBytes == 0 {
		p.pending = pendingPublication{}
		return nil
	}
	bytes := p.spool.acceptedBytes
	callbacks := p.spool.acceptedCallbacks
	records := p.spool.acceptedRecords
	result := p.client.sendAssisted(p.spool.reader(), bytes)
	if !result.succeeded() {
		failure := newSpoolFailure(result.Class, "%v", result.Err)
		p.pending = pendingPublication{Phase: pendingUpload, Class: failure.Class, Err: failure.Err}
		logError("%s publication failed table=%q spool=%q accepted_bytes=%d callbacks=%d records=%d request_source_bytes=%d request_sha256=%s body_reads=%d: %v",
			reason, p.config.Table, p.config.SpoolPath, bytes, callbacks, records,
			result.SourceBytes, result.SourceHash, result.BodyReads, result.Err)
		return failure
	}
	if failure := p.spool.retire(); failure != nil {
		p.pending = pendingPublication{Phase: pendingRetirement, Class: failure.Class, Err: failure.Err}
		logError("%s publication acknowledged but retirement is pending table=%q spool=%q bytes=%d callbacks=%d records=%d sha256=%s: %v",
			reason, p.config.Table, p.config.SpoolPath, bytes, callbacks, records,
			result.SourceHash, failure.Err)
		return failure
	}
	p.pending = pendingPublication{}
	logInfo("%s publication acknowledged table=%q bytes=%d callbacks=%d records=%d sha256=%s body_reads=%d",
		reason, p.config.Table, result.SourceBytes, callbacks, records, result.SourceHash, result.BodyReads)
	return nil
}

func (p *pluginContext) resolvePending() *spoolFailure {
	switch p.pending.Phase {
	case pendingNone:
		return nil
	case pendingUpload:
		if p.pending.Class == failurePermanent {
			return newSpoolFailure(failurePermanent, "%v", p.pending.Err)
		}
		logInfo("retrying pending whole-spool publication table=%q spool=%q bytes=%d callbacks=%d records=%d",
			p.config.Table, p.config.SpoolPath, p.spool.acceptedBytes,
			p.spool.acceptedCallbacks, p.spool.acceptedRecords)
		return p.publishAccepted("pending retry")
	case pendingRetirement:
		failure := p.spool.retire()
		if failure != nil {
			p.pending.Class = failure.Class
			p.pending.Err = failure.Err
			return failure
		}
		p.pending = pendingPublication{}
		logInfo("completed pending durable spool retirement table=%q spool=%q",
			p.config.Table, p.config.SpoolPath)
		return nil
	default:
		return newSpoolFailure(failurePermanent, "invalid pending publication state")
	}
}

func failureCode(class failureClass) int {
	if class == failurePermanent {
		return output.FLB_ERROR
	}
	return output.FLB_RETRY
}

func logInfo(format string, arguments ...interface{}) {
	fmt.Fprintf(os.Stderr, "[manticore] "+format+"\n", arguments...)
}

func logError(format string, arguments ...interface{}) {
	fmt.Fprintf(os.Stderr, "[manticore] error: "+format+"\n", arguments...)
}

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	maxResponseBytes = int64(8 * 1024 * 1024)
	userAgent        = "manticore-fluent-bit-go/1"
)

type failureClass int

const (
	failureNone failureClass = iota
	failureRetryable
	failurePermanent
)

type sendResult struct {
	Class       failureClass
	Err         error
	StatusCode  int
	Response    []byte
	SourceBytes int64
	SourceHash  string
	BodyReads   int64
}

func (r sendResult) succeeded() bool {
	return r.Err == nil && r.Class == failureNone
}

type bulkClient struct {
	config      pluginConfig
	ordinaryURL string
	assistedURL string
	httpClient  *http.Client
	transport   *http.Transport
}

func newBulkClient(config pluginConfig) *bulkClient {
	dialer := &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   connectTimeout,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    true,
		TLSNextProto:          make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}
	return &bulkClient{
		config:      config,
		ordinaryURL: makeBulkURL(config.Endpoint, ""),
		assistedURL: makeBulkURL(config.Endpoint, config.Table),
		httpClient:  &http.Client{Transport: transport},
		transport:   transport,
	}
}

func makeBulkURL(endpoint *url.URL, assistedTable string) string {
	target := *endpoint
	escapedPath := strings.TrimRight(target.EscapedPath(), "/") + "/bulk"
	// EscapedPath always supplies valid escaping, including an encoded final slash.
	target.Path, _ = url.PathUnescape(escapedPath)
	target.RawPath = escapedPath
	target.RawQuery = ""
	if assistedTable != "" {
		query := url.Values{}
		query.Set("bulk_import", assistedTable)
		target.RawQuery = query.Encode()
	}
	return target.String()
}

func (c *bulkClient) close() {
	c.transport.CloseIdleConnections()
}

func (c *bulkClient) sendAssisted(reader io.Reader, sourceBytes int64) sendResult {
	body := newCappedBody(reader, c.config.RequestMaxBytes)
	request, cancel, err := c.newRequest(c.assistedURL, body)
	if err != nil {
		return sendResult{Class: failurePermanent, Err: err}
	}
	defer cancel()
	request.Close = true
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	result := c.do(request)
	result.SourceBytes, result.SourceHash, result.BodyReads = body.snapshot()
	if result.succeeded() && result.SourceBytes != sourceBytes {
		result.Class = failureRetryable
		result.Err = fmt.Errorf("assisted request consumed %d of %d source bytes", result.SourceBytes, sourceBytes)
	}
	return result
}

func (c *bulkClient) sendOrdinary(body []byte, gzip bool) sendResult {
	reader := bytes.NewReader(body)
	request, cancel, err := c.newRequest(c.ordinaryURL, reader)
	if err != nil {
		return sendResult{Class: failurePermanent, Err: err}
	}
	defer cancel()
	request.ContentLength = int64(len(body))
	if gzip {
		request.Header.Set("Content-Encoding", "gzip")
	}
	return c.do(request)
}

func (c *bulkClient) newRequest(target string, body io.Reader) (*http.Request, context.CancelFunc, error) {
	requestContext := context.Background()
	cancel := func() {}
	if c.config.RequestTimeout > 0 {
		requestContext, cancel = context.WithTimeout(requestContext, c.config.RequestTimeout)
	}
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, target, body)
	if err != nil {
		cancel()
		return nil, func() {}, fmt.Errorf("could not create Manticore bulk request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-ndjson")
	request.Header.Set("User-Agent", userAgent)
	if c.config.HTTPUser != "" {
		request.SetBasicAuth(c.config.HTTPUser, c.config.HTTPPassword)
	}
	return request, cancel, nil
}

func (c *bulkClient) do(request *http.Request) sendResult {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return sendResult{Class: failureRetryable, Err: fmt.Errorf("Manticore bulk request failed: %w", err)}
	}
	defer response.Body.Close()
	limited := &io.LimitedReader{R: response.Body, N: maxResponseBytes + 1}
	body, err := ioutil.ReadAll(limited)
	if err != nil {
		return sendResult{
			Class: failureRetryable, Err: fmt.Errorf("could not read Manticore bulk response: %w", err),
			StatusCode: response.StatusCode,
		}
	}
	if int64(len(body)) > maxResponseBytes {
		return sendResult{
			Class: failureRetryable, Err: fmt.Errorf("Manticore bulk response exceeds %d bytes", maxResponseBytes),
			StatusCode: response.StatusCode,
		}
	}
	result := classifyResponse(response.StatusCode, body)
	result.StatusCode = response.StatusCode
	result.Response = body
	return result
}

type responseEnvelope struct {
	Errors *bool             `json:"errors"`
	Items  []json.RawMessage `json:"items"`
}

func classifyResponse(statusCode int, body []byte) sendResult {
	envelope := responseEnvelope{}
	parseErr := json.Unmarshal(body, &envelope)
	failedItem, retryableItem, validItems := inspectItemStatuses(envelope.Items)

	if statusCode >= 200 && statusCode < 300 {
		if parseErr == nil && envelope.Errors != nil && !*envelope.Errors && validItems && !failedItem {
			return sendResult{}
		}
		if parseErr == nil && envelope.Errors != nil && retryableItem {
			return sendResult{Class: failureRetryable, Err: fmt.Errorf("Manticore /bulk returned a retryable item error")}
		}
		return sendResult{Class: failurePermanent, Err: fmt.Errorf("invalid or permanently failed Manticore /bulk response (HTTP %d)", statusCode)}
	}

	if retryableItem || statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests {
		return sendResult{Class: failureRetryable, Err: fmt.Errorf("Manticore /bulk returned HTTP %d", statusCode)}
	}
	if failedItem {
		return sendResult{Class: failurePermanent, Err: fmt.Errorf("Manticore /bulk returned HTTP %d", statusCode)}
	}
	if statusCode >= 500 {
		return sendResult{Class: failureRetryable, Err: fmt.Errorf("Manticore /bulk returned HTTP %d", statusCode)}
	}
	return sendResult{Class: failurePermanent, Err: fmt.Errorf("Manticore /bulk returned HTTP %d", statusCode)}
}

func inspectItemStatuses(items []json.RawMessage) (failed, retryable, valid bool) {
	valid = true
	for _, item := range items {
		var actions map[string]struct {
			Status *int `json:"status"`
		}
		if json.Unmarshal(item, &actions) != nil || len(actions) != 1 {
			valid = false
			continue
		}
		for _, action := range actions {
			if action.Status == nil {
				valid = false
				continue
			}
			status := *action.Status
			if status < 200 || status >= 300 {
				failed = true
			}
			if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500 {
				retryable = true
			}
		}
	}
	return
}

type cappedBody struct {
	reader io.Reader
	cap    int
	mu     sync.Mutex // net/http can still read the body after an early response.
	hasher hash.Hash
	bytes  int64
	reads  int64
}

func newCappedBody(reader io.Reader, capBytes int64) *cappedBody {
	maximumInt := int64(^uint(0) >> 1)
	if capBytes > maximumInt {
		capBytes = maximumInt
	}
	return &cappedBody{reader: reader, cap: int(capBytes), hasher: sha256.New()}
}

func (b *cappedBody) Read(destination []byte) (int, error) {
	if len(destination) > b.cap {
		destination = destination[:b.cap]
	}
	written, err := b.reader.Read(destination)
	if written > 0 {
		b.mu.Lock()
		b.reads++
		b.bytes += int64(written)
		_, _ = b.hasher.Write(destination[:written])
		b.mu.Unlock()
	}
	return written, err
}

func (b *cappedBody) Close() error {
	return nil
}

func (b *cappedBody) snapshot() (int64, string, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bytes, hex.EncodeToString(b.hasher.Sum(nil)), b.reads
}

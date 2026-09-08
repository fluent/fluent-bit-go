package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

const (
	defaultEndpoint        = "http://127.0.0.1:9308"
	defaultIDKey           = "id"
	defaultRequestMaxBytes = int64(64 * 1024)
	connectTimeout         = 10 * time.Second
)

type deliveryMode int

const (
	modeOrdinary deliveryMode = iota
	modeAssisted
)

type pluginConfig struct {
	Endpoint           *url.URL
	Table              string
	IDKey              string
	Action             string
	Mode               deliveryMode
	CompressGZIP       bool
	SpoolPath          string
	RequestMaxBytes    int64
	PublishSourceBytes int64
	RequestTimeout     time.Duration
	HTTPUser           string
	HTTPPassword       string
}

func readConfig(plugin unsafe.Pointer) (pluginConfig, error) {
	get := func(key string) string {
		return strings.TrimSpace(output.FLBPluginConfigKey(plugin, key))
	}

	endpointText := get("Endpoint")
	if endpointText == "" {
		endpointText = defaultEndpoint
	}
	endpoint, err := parseEndpoint(endpointText)
	if err != nil {
		return pluginConfig{}, err
	}

	table := get("Table")
	if table == "" {
		return pluginConfig{}, fmt.Errorf("Table is required")
	}

	idKey := get("Id_Key")
	if idKey == "" {
		idKey = defaultIDKey
	}

	action := get("Action")
	if action == "" {
		action = "insert"
	}
	if !strings.EqualFold(action, "insert") {
		return pluginConfig{}, fmt.Errorf("Action must be 'insert'")
	}
	action = "insert"

	chunked, err := parseOnOff("Chunked", get("Chunked"), false)
	if err != nil {
		return pluginConfig{}, err
	}
	mode := modeOrdinary
	if chunked {
		mode = modeAssisted
	}

	compress := get("Compress")
	compressGZIP := false
	if compress != "" {
		if !strings.EqualFold(compress, "gzip") {
			return pluginConfig{}, fmt.Errorf("Compress must be 'gzip' or omitted")
		}
		compressGZIP = true
	}
	if mode == modeAssisted && compressGZIP {
		return pluginConfig{}, fmt.Errorf("Compress gzip is not supported with Chunked On")
	}

	spoolPath := get("Spool_Path")
	if mode == modeAssisted && spoolPath == "" {
		return pluginConfig{}, fmt.Errorf("Spool_Path is required with Chunked On")
	}

	requestMaxBytes, err := parseSize("Request_Max_Bytes", get("Request_Max_Bytes"), defaultRequestMaxBytes, false)
	if err != nil {
		return pluginConfig{}, err
	}
	publishSourceBytes, err := parseSize("Publish_Source_Bytes", get("Publish_Source_Bytes"), 0, true)
	if err != nil {
		return pluginConfig{}, err
	}

	requestTimeout, err := parseRequestTimeout(get("Request_Timeout"))
	if err != nil {
		return pluginConfig{}, err
	}

	httpUser := get("HTTP_User")
	httpPassword := output.FLBPluginConfigKey(plugin, "HTTP_Passwd")
	if httpUser == "" && httpPassword != "" {
		return pluginConfig{}, fmt.Errorf("HTTP_Passwd requires HTTP_User")
	}

	return pluginConfig{
		Endpoint:           endpoint,
		Table:              table,
		IDKey:              idKey,
		Action:             action,
		Mode:               mode,
		CompressGZIP:       compressGZIP,
		SpoolPath:          spoolPath,
		RequestMaxBytes:    requestMaxBytes,
		PublishSourceBytes: publishSourceBytes,
		RequestTimeout:     requestTimeout,
		HTTPUser:           httpUser,
		HTTPPassword:       httpPassword,
	}, nil
}

func parseEndpoint(text string) (*url.URL, error) {
	endpoint, err := url.Parse(text)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("Endpoint must be an absolute HTTP or HTTPS URL")
	}
	endpoint.Scheme = strings.ToLower(endpoint.Scheme)
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("Endpoint scheme must be http or https")
	}
	if endpoint.User != nil {
		return nil, fmt.Errorf("Endpoint must not contain credentials; use HTTP_User and HTTP_Passwd")
	}
	if endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("Endpoint must not contain a query or fragment")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/")
	return endpoint, nil
}

func parseOnOff(name, text string, defaultValue bool) (bool, error) {
	if text == "" {
		return defaultValue, nil
	}
	switch strings.ToLower(text) {
	case "on", "true":
		return true, nil
	case "off", "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be On or Off", name)
	}
}

func parseSize(name, text string, defaultValue int64, allowZero bool) (int64, error) {
	if text == "" {
		return defaultValue, nil
	}

	valueText := strings.TrimSpace(strings.ToUpper(text))
	multiplier := int64(1)
	for _, suffix := range []struct {
		text       string
		multiplier int64
	}{
		{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"TIB", 1 << 40},
		{"KB", 1 << 10}, {"MB", 1 << 20}, {"GB", 1 << 30}, {"TB", 1 << 40},
		{"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30}, {"T", 1 << 40},
		{"B", 1},
	} {
		if strings.HasSuffix(valueText, suffix.text) {
			multiplier = suffix.multiplier
			valueText = strings.TrimSpace(strings.TrimSuffix(valueText, suffix.text))
			break
		}
	}
	value, err := strconv.ParseInt(valueText, 10, 64)
	if err != nil || value < 0 || (value == 0 && !allowZero) {
		if allowZero {
			return 0, fmt.Errorf("%s must be zero or a positive byte size", name)
		}
		return 0, fmt.Errorf("%s must be a positive byte size", name)
	}
	if value > 0 && value > (int64(^uint64(0)>>1))/multiplier {
		return 0, fmt.Errorf("%s is too large", name)
	}
	return value * multiplier, nil
}

func parseRequestTimeout(text string) (time.Duration, error) {
	if text == "" || text == "0" {
		return 0, nil
	}
	if seconds, err := strconv.ParseInt(text, 10, 64); err == nil {
		if seconds <= 0 || seconds > int64((1<<63-1)/time.Second) {
			return 0, fmt.Errorf("Request_Timeout must be zero or a positive duration")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	duration, err := time.ParseDuration(text)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("Request_Timeout must be zero or a positive duration such as 30s")
	}
	return duration, nil
}

func (c pluginConfig) modeName() string {
	if c.Mode == modeAssisted {
		return "assisted"
	}
	if c.CompressGZIP {
		return "ordinary-gzip"
	}
	return "ordinary-fixed"
}

func (c pluginConfig) endpointForLog() string {
	copy := *c.Endpoint
	copy.User = nil
	return copy.String()
}

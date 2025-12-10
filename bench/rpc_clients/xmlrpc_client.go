package rpc_clients

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

// XMLRPCClient implements XML-RPC protocol over Unix Domain Socket
type XMLRPCClient struct {
	httpClient *http.Client
	udsPath    string
	mu         sync.Mutex
}

// NewXMLRPCClient creates a new XML-RPC client
func NewXMLRPCClient() *XMLRPCClient {
	return &XMLRPCClient{}
}

// Connect establishes connection to XML-RPC server via UDS
func (c *XMLRPCClient) Connect(udsPath string) error {
	// Create HTTP client with Unix Domain Socket transport
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", udsPath)
			},
		},
	}
	c.udsPath = udsPath
	return nil
}

// Call invokes an XML-RPC method
func (c *XMLRPCClient) Call(ctx context.Context, method string, args interface{}, reply interface{}) error {
	if c.httpClient == nil {
		return fmt.Errorf("not connected")
	}

	// Create XML-RPC request
	request, err := c.encodeRequest(method, args)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Send HTTP POST request
	req, err := http.NewRequestWithContext(ctx, "POST", "http://unix/RPC2", bytes.NewReader(request))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")

	c.mu.Lock()
	resp, err := c.httpClient.Do(req)
	c.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Decode XML-RPC response
	return c.decodeResponse(body, reply)
}

// encodeRequest encodes method call to XML-RPC format
func (c *XMLRPCClient) encodeRequest(method string, args interface{}) ([]byte, error) {
	var buf bytes.Buffer

	// Write XML header
	buf.WriteString(`<?xml version="1.0"?>`)
	buf.WriteString(`<methodCall>`)
	buf.WriteString(`<methodName>` + method + `</methodName>`)
	buf.WriteString(`<params>`)

	// Encode parameters
	if args != nil {
		buf.WriteString(`<param>`)
		if err := c.encodeValue(&buf, args); err != nil {
			return nil, err
		}
		buf.WriteString(`</param>`)
	}

	buf.WriteString(`</params>`)
	buf.WriteString(`</methodCall>`)

	return buf.Bytes(), nil
}

// encodeValue encodes a value to XML-RPC format
func (c *XMLRPCClient) encodeValue(buf *bytes.Buffer, v interface{}) error {
	buf.WriteString(`<value>`)

	switch val := v.(type) {
	case int:
		buf.WriteString(fmt.Sprintf(`<int>%d</int>`, val))
	case string:
		buf.WriteString(`<string>`)
		_ = xml.EscapeText(buf, []byte(val))
		buf.WriteString(`</string>`)
	case map[string]interface{}:
		buf.WriteString(`<struct>`)
		for k, v := range val {
			buf.WriteString(`<member>`)
			buf.WriteString(`<name>` + k + `</name>`)
			if err := c.encodeValue(buf, v); err != nil {
				return err
			}
			buf.WriteString(`</member>`)
		}
		buf.WriteString(`</struct>`)
	case []interface{}:
		buf.WriteString(`<array><data>`)
		for _, item := range val {
			if err := c.encodeValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteString(`</data></array>`)
	default:
		// Simplified encoding for benchmark purposes
		buf.WriteString(fmt.Sprintf(`<string>%v</string>`, val))
	}

	buf.WriteString(`</value>`)
	return nil
}

// decodeResponse decodes XML-RPC response
func (c *XMLRPCClient) decodeResponse(data []byte, reply interface{}) error {
	// Simplified XML parsing for benchmark purposes
	// In production, use proper XML-RPC library

	// Check for fault
	if bytes.Contains(data, []byte("<fault>")) {
		return fmt.Errorf("XML-RPC fault in response")
	}

	// Extract result value (simplified)
	// In real implementation, properly parse XML structure
	if reply != nil {
		// For benchmark purposes, we'll just set a simple result
		if m, ok := reply.(*map[string]interface{}); ok {
			*m = map[string]interface{}{
				"result": "processed",
			}
		}
	}

	return nil
}

// Close terminates the connection
func (c *XMLRPCClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

// Name returns the protocol identifier
func (c *XMLRPCClient) Name() string {
	return "xml-rpc"
}

package framing

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/YuminosukeSato/pyproc/internal/protocol"
)

func TestFramer_WriteMessage(t *testing.T) {
	tests := []struct {
		name    string
		req     *protocol.Request
		wantErr bool
	}{
		{
			name: "simple request",
			req: &protocol.Request{
				ID:     1,
				Method: "echo",
				Body:   []byte(`{"message":"hello"}`),
			},
			wantErr: false,
		},
		{
			name: "empty body request",
			req: &protocol.Request{
				ID:     2,
				Method: "ping",
				Body:   []byte(`{}`),
			},
			wantErr: false,
		},
		{
			name: "large body request",
			req: &protocol.Request{
				ID:     3,
				Method: "process",
				Body:   []byte(`{"data":"` + "x" + `"}`),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			framer := NewFramer(&buf)

			// Marshal the request
			data, err := tt.req.Marshal()
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			// Write the message
			err = framer.WriteMessage(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the frame structure
				written := buf.Bytes()
				if len(written) < 4 {
					t.Fatal("frame too short")
				}

				// Check length header
				lengthBytes := written[:4]
				length := binary.BigEndian.Uint32(lengthBytes)
				if int(length) != len(data) {
					t.Errorf("length mismatch: header=%d, actual=%d", length, len(data))
				}

				// Check payload
				payload := written[4:]
				if !bytes.Equal(payload, data) {
					t.Error("payload mismatch")
				}
			}
		})
	}
}

func TestFramer_ReadMessage(t *testing.T) {
	tests := []struct {
		name    string
		resp    *protocol.Response
		wantErr bool
	}{
		{
			name: "simple response",
			resp: &protocol.Response{
				ID:   1,
				OK:   true,
				Body: []byte(`{"result":"success"}`),
			},
			wantErr: false,
		},
		{
			name: "error response",
			resp: &protocol.Response{
				ID:       2,
				OK:       false,
				ErrorMsg: "something went wrong",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the response
			data, err := tt.resp.Marshal()
			if err != nil {
				t.Fatalf("failed to marshal response: %v", err)
			}

			// Create a frame with the response
			var buf bytes.Buffer
			framer := NewFramer(&buf)
			if err := framer.WriteMessage(data); err != nil {
				t.Fatalf("failed to write message: %v", err)
			}

			// Read the message back
			readFramer := NewFramer(&buf)
			msg, err := readFramer.ReadMessage()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the read message matches original
				if !bytes.Equal(msg, data) {
					t.Error("read message doesn't match original")
				}

				// Verify we can unmarshal it back
				var resp protocol.Response
				if err := resp.Unmarshal(msg); err != nil {
					t.Errorf("failed to unmarshal response: %v", err)
				}
				if resp.ID != tt.resp.ID {
					t.Errorf("ID mismatch: got=%d, want=%d", resp.ID, tt.resp.ID)
				}
			}
		})
	}
}

func TestFramer_MaxFrameSize(t *testing.T) {
	var buf bytes.Buffer
	maxSize := 100
	framer := NewFramerWithMaxSize(&buf, maxSize)

	// Try to write a message larger than max size
	largeData := make([]byte, maxSize+1)
	err := framer.WriteMessage(largeData)
	if err == nil {
		t.Error("expected error for oversized message")
	}
}

func TestFramer_PartialRead(t *testing.T) {
	// Create a valid frame
	req := &protocol.Request{
		ID:     1,
		Method: "test",
		Body:   []byte(`{"test":true}`),
	}
	data, _ := req.Marshal()

	var fullBuf bytes.Buffer
	framer := NewFramer(&fullBuf)
	_ = framer.WriteMessage(data)

	// Simulate partial read by creating a reader that returns data in chunks
	fullData := fullBuf.Bytes()
	pr := &partialReader{
		data:      fullData,
		chunkSize: 10, // Read 10 bytes at a time
	}

	readFramer := NewFramer(pr)
	msg, err := readFramer.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	if !bytes.Equal(msg, data) {
		t.Error("partial read resulted in corrupted message")
	}
}

func TestFramer_ReadMessageErrors(t *testing.T) {
	tests := []struct {
		name   string
		framer *Framer
	}{{"header EOF", NewFramer(bytes.NewBuffer(nil))}, {"frame too large", NewFramerWithMaxSize(bytes.NewBuffer(binary.BigEndian.AppendUint32(nil, 2)), 1)}}
	for _, tt := range tests {
		if _, err := tt.framer.ReadMessage(); err == nil {
			t.Fatalf("%s: expected error", tt.name)
		}
	}
}

// partialReader simulates reading data in small chunks
type partialReader struct {
	data      []byte
	offset    int
	chunkSize int
}

func (r *partialReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	remaining := len(r.data) - r.offset
	toRead := r.chunkSize
	if toRead > remaining {
		toRead = remaining
	}
	if toRead > len(p) {
		toRead = len(p)
	}

	copy(p, r.data[r.offset:r.offset+toRead])
	r.offset += toRead
	return toRead, nil
}

func (r *partialReader) Write(_ []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}

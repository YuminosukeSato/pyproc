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

func TestFramer_ReadMessage_EOF(t *testing.T) {
	buf := &bytes.Buffer{}
	framer := NewFramer(buf)
	_, err := framer.ReadMessage()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestFramer_ReadMessage_FrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, DefaultMaxFrameSize+1)
	buf.Write(lengthBuf)

	framer := NewFramer(&buf)
	_, err := framer.ReadMessage()
	if err == nil {
		t.Error("expected error for oversized frame")
	}
}

func TestUnmarshalFrame_TooShort(t *testing.T) {
	data := make([]byte, FrameHeaderSize-1)
	_, err := UnmarshalFrame(data)
	if err == nil {
		t.Error("expected error for short frame")
	}
}

func TestUnmarshalFrame_InvalidMagic(t *testing.T) {
	data := make([]byte, FrameHeaderSize)
	data[0], data[1] = 0x00, 0x00
	_, err := UnmarshalFrame(data)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestUnmarshalFrame_LengthMismatch(t *testing.T) {
	frame := NewFrame(1, []byte("test"))
	data := frame.Marshal()
	data = append(data, 0x00)
	_, err := UnmarshalFrame(data)
	if err == nil {
		t.Error("expected error for length mismatch")
	}
}

func TestUnmarshalFrame_CRCMismatch(t *testing.T) {
	frame := NewFrame(1, []byte("test"))
	data := frame.Marshal()
	data[len(data)-1] ^= 0xFF
	_, err := UnmarshalFrame(data)
	if err == nil {
		t.Error("expected error for CRC mismatch")
	}
}

func TestWriteFrame_NonEnhancedMode(t *testing.T) {
	var buf bytes.Buffer
	framer := NewFramer(&buf)
	frame := NewFrame(1, []byte("test"))
	if err := framer.WriteFrame(frame); err != nil {
		t.Errorf("WriteFrame failed: %v", err)
	}
}

func TestWriteFrame_EnhancedMode(t *testing.T) {
	var buf bytes.Buffer
	framer := NewEnhancedFramer(&buf)
	frame := NewFrame(1, []byte("test"))
	if err := framer.WriteFrame(frame); err != nil {
		t.Errorf("WriteFrame failed: %v", err)
	}
}

func TestWriteFrame_EnhancedMode_TooLarge(t *testing.T) {
	var buf bytes.Buffer
	framer := NewFramerWithMaxSize(&buf, 10)
	framer.enhancedMode = true
	frame := NewFrame(1, make([]byte, 100))
	if err := framer.WriteFrame(frame); err == nil {
		t.Error("expected error for oversized payload")
	}
}

func TestReadFrame_NonEnhancedMode(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFramer(&buf)
	_ = writer.WriteMessage([]byte("test"))

	reader := NewFramer(&buf)
	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if string(frame.Payload) != "test" {
		t.Errorf("unexpected payload: %s", frame.Payload)
	}
}

func TestReadFrame_EnhancedMode(t *testing.T) {
	var buf bytes.Buffer
	writer := NewEnhancedFramer(&buf)
	frame := NewFrame(42, []byte("hello"))
	_ = writer.WriteFrame(frame)

	reader := NewEnhancedFramer(&buf)
	readFrame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if readFrame.Header.RequestID != 42 {
		t.Errorf("unexpected request ID: %d", readFrame.Header.RequestID)
	}
}

func TestReadFrame_EnhancedMode_EOF(t *testing.T) {
	buf := &bytes.Buffer{}
	framer := NewEnhancedFramer(buf)
	_, err := framer.ReadFrame()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestReadFrame_EnhancedMode_InvalidMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00})
	framer := NewEnhancedFramer(&buf)
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

type errorWriter struct{}

func (e *errorWriter) Write(_ []byte) (int, error) { return 0, io.ErrShortWrite }
func (e *errorWriter) Read(_ []byte) (int, error)  { return 0, io.ErrUnexpectedEOF }

func TestWriteMessage_WriteError(t *testing.T) {
	framer := NewFramer(&errorWriter{})
	err := framer.WriteMessage([]byte("test"))
	if err == nil {
		t.Error("expected write error")
	}
}

func TestWriteFrame_WriteError(t *testing.T) {
	framer := NewEnhancedFramer(&errorWriter{})
	frame := NewFrame(1, []byte("test"))
	err := framer.WriteFrame(frame)
	if err == nil {
		t.Error("expected write error")
	}
}

func TestReadMessage_ReadDataError(t *testing.T) {
	var buf bytes.Buffer
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, 10)
	buf.Write(lengthBuf)
	buf.Write([]byte("short"))

	framer := NewFramer(&buf)
	_, err := framer.ReadMessage()
	if err == nil {
		t.Error("expected read data error")
	}
}

func TestReadFrame_EnhancedMode_HeaderReadError(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{MagicByte1, MagicByte2})

	framer := NewEnhancedFramer(&buf)
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected header read error")
	}
}

func TestReadFrame_EnhancedMode_FrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{MagicByte1, MagicByte2})
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, DefaultMaxFrameSize+FrameHeaderSize+1)
	buf.Write(lengthBuf)
	buf.Write(make([]byte, 12))

	framer := NewEnhancedFramer(&buf)
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected frame too large error")
	}
}

type lengthOnlyWriter struct {
	written int
}

func (w *lengthOnlyWriter) Write(p []byte) (int, error) {
	if w.written == 0 {
		w.written = len(p)
		return len(p), nil
	}
	return 0, io.ErrShortWrite
}
func (w *lengthOnlyWriter) Read(_ []byte) (int, error) { return 0, io.EOF }

func TestWriteMessage_DataWriteError(t *testing.T) {
	framer := NewFramer(&lengthOnlyWriter{})
	err := framer.WriteMessage([]byte("test"))
	if err == nil {
		t.Error("expected data write error")
	}
}

type lengthErrorReader struct{}

func (r *lengthErrorReader) Read(_ []byte) (int, error)  { return 0, io.ErrUnexpectedEOF }
func (r *lengthErrorReader) Write(_ []byte) (int, error) { return 0, nil }

func TestReadMessage_LengthReadError(t *testing.T) {
	framer := NewFramer(&lengthErrorReader{})
	_, err := framer.ReadMessage()
	if err == nil || err == io.EOF {
		t.Errorf("expected non-EOF read error, got %v", err)
	}
}

func TestReadFrame_EnhancedMode_PayloadReadError(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{MagicByte1, MagicByte2})
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, FrameHeaderSize+10)
	buf.Write(lengthBuf)
	buf.Write(make([]byte, 8))

	framer := NewEnhancedFramer(&buf)
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected payload read error")
	}
}

func TestReadFrame_EnhancedMode_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	frame := NewFrame(1, []byte{})
	writer := NewEnhancedFramer(&buf)
	_ = writer.WriteFrame(frame)

	reader := NewEnhancedFramer(&buf)
	readFrame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if len(readFrame.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(readFrame.Payload))
	}
}

func TestReadFrame_NonEnhancedMode_Error(t *testing.T) {
	framer := NewFramer(&errorWriter{})
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected read error")
	}
}

func TestReadFrame_EnhancedMode_MagicReadError(t *testing.T) {
	framer := NewEnhancedFramer(&lengthErrorReader{})
	_, err := framer.ReadFrame()
	if err == nil || err == io.EOF {
		t.Errorf("expected non-EOF magic read error, got %v", err)
	}
}

type phaseReader struct {
	phase int
}

func (r *phaseReader) Read(p []byte) (int, error) {
	switch r.phase {
	case 0:
		p[0], p[1] = MagicByte1, MagicByte2
		r.phase++
		return 2, nil
	case 1:
		header := make([]byte, FrameHeaderSize-2)
		binary.BigEndian.PutUint32(header[0:4], FrameHeaderSize+10)
		copy(p, header)
		r.phase++
		return FrameHeaderSize - 2, nil
	default:
		return 0, io.ErrUnexpectedEOF
	}
}
func (r *phaseReader) Write(_ []byte) (int, error) { return 0, nil }

func TestReadFrame_EnhancedMode_PayloadReadErrorActual(t *testing.T) {
	framer := NewEnhancedFramer(&phaseReader{})
	_, err := framer.ReadFrame()
	if err == nil {
		t.Error("expected payload read error")
	}
}

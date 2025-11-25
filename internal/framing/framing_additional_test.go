package framing

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestWriteMessageTooLarge(t *testing.T) {
	buf := &bytes.Buffer{}
	framer := NewFramerWithMaxSize(buf, 2)
	if err := framer.WriteMessage([]byte{1, 2, 3}); err == nil {
		t.Fatalf("expected error for oversized message")
	}
}

func TestReadMessageTooLarge(t *testing.T) {
	buf := &bytes.Buffer{}
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, 5)
	buf.Write(length)
	buf.Write([]byte{1, 2, 3, 4, 5})

	framer := NewFramerWithMaxSize(buf, 2)
	_, err := framer.ReadMessage()
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected max frame size error, got %v", err)
	}
}

func TestReadFrameInvalidMagic(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, 0x05, 0x00, 0x00})
	framer := NewEnhancedFramer(buf)
	if _, err := framer.ReadFrame(); err == nil {
		t.Fatalf("expected invalid magic error")
	}
}

func TestEnhancedFrameRoundTrip(t *testing.T) {
	frame := NewFrame(7, []byte("hello"))
	writer := &bytes.Buffer{}
	framer := NewEnhancedFramer(writer)
	if err := framer.WriteFrame(frame); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	reader := NewEnhancedFramer(bytes.NewBuffer(writer.Bytes()))
	decoded, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if decoded.Header.RequestID != 7 || string(decoded.Payload) != "hello" {
		t.Fatalf("unexpected frame: %+v", decoded)
	}
}

func TestEnhancedFrameTooLarge(t *testing.T) {
	frame := NewFrame(1, []byte{1, 2, 3, 4})
	buf := &bytes.Buffer{}
	framer := NewEnhancedFramer(buf)
	framer.maxFrameSize = 2
	if err := framer.WriteFrame(frame); err == nil {
		t.Fatalf("expected error for oversized frame")
	}
}

func TestEnhancedFrameLengthExceedsMax(t *testing.T) {
	buf := &bytes.Buffer{}
	framer := NewEnhancedFramer(buf)
	framer.maxFrameSize = 2
	frame := NewFrame(1, []byte{1, 2, 3, 4})
	buf.Write(frame.Marshal())
	reader := NewEnhancedFramer(bytes.NewBuffer(buf.Bytes()))
	reader.maxFrameSize = 2
	if _, err := reader.ReadFrame(); err == nil {
		t.Fatalf("expected frame size error")
	}
}

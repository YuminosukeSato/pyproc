package protocol

import (
	"encoding/json"
	"testing"
)

func TestRequestLifecycle(t *testing.T) {
	req, err := NewRequest(42, "echo", map[string]string{"msg": "hi"})
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	data, err := req.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Request
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	var body map[string]string
	if err := decoded.UnmarshalBody(&body); err != nil {
		t.Fatalf("UnmarshalBody failed: %v", err)
	}

	if body["msg"] != "hi" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestResponseLifecycle(t *testing.T) {
	resp, err := NewResponse(7, map[string]int{"v": 1})
	if err != nil {
		t.Fatalf("NewResponse failed: %v", err)
	}

	data, err := resp.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Response
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	var body map[string]int
	if err := decoded.UnmarshalBody(&body); err != nil {
		t.Fatalf("UnmarshalBody failed: %v", err)
	}

	if body["v"] != 1 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestErrorResponse(t *testing.T) {
	errResp := NewErrorResponse(9, json.Unmarshal([]byte("{"), &struct{}{}))
	if errResp.OK {
		t.Fatal("expected error response")
	}

	err := errResp.Error()
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	errResp.ErrorMsg = ""
	if err := errResp.Error(); err == nil {
		t.Fatal("expected error when OK is false without message")
	}
}

func TestResponseErrorOK(t *testing.T) {
	resp := Response{OK: true, ErrorMsg: "ignored"}
	if err := resp.Error(); err != nil {
		t.Fatalf("expected nil error for OK response, got %v", err)
	}
}

func TestCancellationRequest(t *testing.T) {
	cr := NewCancellationRequest(5, "timeout")
	if cr.ID != 5 || cr.Reason != "timeout" {
		t.Fatalf("unexpected cancellation request: %+v", cr)
	}
}

func TestWrapAndUnwrap(t *testing.T) {
	req := Request{ID: 1, Method: "m", Body: []byte(`{"a":1}`)}
	msg, err := WrapMessage(MessageTypeRequest, req)
	if err != nil {
		t.Fatalf("WrapMessage failed: %v", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message failed: %v", err)
	}

	unwrapped, err := UnwrapMessage(data)
	if err != nil {
		t.Fatalf("UnwrapMessage failed: %v", err)
	}

	if unwrapped.Type != MessageTypeRequest {
		t.Fatalf("unexpected type: %s", unwrapped.Type)
	}

	var roundTrip Request
	if err := json.Unmarshal(unwrapped.Payload, &roundTrip); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if roundTrip.Method != "m" || roundTrip.ID != 1 {
		t.Fatalf("unexpected payload: %+v", roundTrip)
	}
}

func TestRequestUnmarshalBodyNil(t *testing.T) {
	req := Request{Body: nil}
	var target map[string]string
	if err := req.UnmarshalBody(&target); err == nil {
		t.Fatal("expected error when body is nil")
	}
}

func TestResponseUnmarshalBodyNil(t *testing.T) {
	resp := Response{OK: true, Body: nil}
	var target map[string]string
	if err := resp.UnmarshalBody(&target); err == nil {
		t.Fatal("expected error when body is nil")
	}
}

func TestRequestResponseMarshalFailures(t *testing.T) {
	if _, err := NewRequest(1, "bad", make(chan int)); err == nil {
		t.Fatal("expected marshal error for request")
	}
	if _, err := NewResponse(1, make(chan int)); err == nil {
		t.Fatal("expected marshal error for response")
	}
}

func TestWrapUnwrapErrors(t *testing.T) {
	if _, err := WrapMessage(MessageTypeRequest, make(chan int)); err == nil {
		t.Fatal("expected wrap marshal error")
	}
	if _, err := UnwrapMessage([]byte("not-json")); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

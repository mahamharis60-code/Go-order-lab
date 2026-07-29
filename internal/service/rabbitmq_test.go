package service

import "testing"

func TestRabbitOrderMessageRoundTrip(t *testing.T) {
	body, err := encodeRabbitOrderMessage(RabbitOrderMessage{OrderNo: "ORD202607290001", Retry: 2})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	msg, err := decodeRabbitOrderMessage(body)
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}

	if msg.OrderNo != "ORD202607290001" {
		t.Fatalf("order no = %q, want %q", msg.OrderNo, "ORD202607290001")
	}
	if msg.Retry != 2 {
		t.Fatalf("retry = %d, want %d", msg.Retry, 2)
	}
}

func TestRabbitOrderMessagePlainTextCompatibility(t *testing.T) {
	msg, err := decodeRabbitOrderMessage([]byte("ORD202607290002"))
	if err != nil {
		t.Fatalf("decode plain order no: %v", err)
	}

	if msg.OrderNo != "ORD202607290002" {
		t.Fatalf("order no = %q, want %q", msg.OrderNo, "ORD202607290002")
	}
	if msg.Retry != 0 {
		t.Fatalf("retry = %d, want 0", msg.Retry)
	}
}

func TestRabbitOrderMessageRejectsEmpty(t *testing.T) {
	_, err := encodeRabbitOrderMessage(RabbitOrderMessage{})
	if err == nil {
		t.Fatal("expected empty order_no to fail")
	}

	_, err = decodeRabbitOrderMessage(nil)
	if err == nil {
		t.Fatal("expected empty payload to fail")
	}
}

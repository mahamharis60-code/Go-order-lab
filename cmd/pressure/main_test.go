package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSummary(t *testing.T) {
	r := summarize("test", 2, 2*time.Second, []sample{
		{Status: 202, Latency: time.Millisecond},
		{Status: 409, Latency: 2 * time.Millisecond},
		{Status: 429, Latency: 3 * time.Millisecond},
		{Status: 500, Latency: 4 * time.Millisecond},
		{Failed: true, Latency: 5 * time.Millisecond},
	})
	if r.QPS != 2.5 || r.AcceptedQPS != .5 || r.P50MS != 3 || r.P99MS != 5 || r.SystemErrors != 2 || r.SystemErrorRate != .4 {
		t.Fatalf("unexpected summary: %+v", r)
	}
}

func TestRequestWorkersBoundConcurrency(t *testing.T) {
	var active, maximum atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if n <= old || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
	}))
	defer server.Close()
	c := client{base: server.URL, http: server.Client()}
	r := runLoad("test", 4, 31, 0, func(int) sample {
		s, _, err := c.request(context.Background(), "GET", "/", "", nil)
		if err != nil {
			t.Error(err)
		}
		return s
	})
	if r.Requests != 31 || r.Statuses[200] != 31 || maximum.Load() > 4 || maximum.Load() < 2 {
		t.Fatalf("report=%+v max=%d", r, maximum.Load())
	}
}

func TestDurationStopsScheduling(t *testing.T) {
	r := runLoad("duration", 2, 0, 30*time.Millisecond, func(int) sample {
		time.Sleep(5 * time.Millisecond)
		return sample{Status: 200, Latency: 5 * time.Millisecond}
	})
	if r.Requests == 0 || r.ElapsedSeconds < .03 || r.ElapsedSeconds > 1 {
		t.Fatalf("unexpected duration result: %+v", r)
	}
}

func TestInvalidJSONIsSystemError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not JSON")) }))
	defer server.Close()
	c := client{base: server.URL, http: server.Client()}
	s, _, err := c.request(context.Background(), "GET", "/", "", nil)
	if err == nil || !s.Failed {
		t.Fatal("invalid JSON must be treated as a failed response")
	}
}

func TestVerificationRejectsInventoryMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/admin/orders":
			if r.URL.Query().Get("status") == "QUEUED" {
				_, _ = w.Write([]byte(`{"data":{"total":0,"items":[]}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"total":1,"items":[{"order_no":"test","user_id":1,"status":"WAIT_PAY"}]}}`))
			}
		case "/api/ops/stock/reconcile":
			_, _ = w.Write([]byte(`{"data":{"checked":1,"mismatched":1,"items":[{"mysql_stock":4,"redis_stock":3,"redis_exists":true}]}}`))
		case "/api/activities":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"stock":4}]}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := client{base: server.URL, http: server.Client()}
	v, _, err := c.verify("test-token", 1, 5, 1)
	if err == nil || v["passed"] != false {
		t.Fatal("inventory drift must fail verification")
	}
}

func TestVerificationAcceptsEmptyDifferenceList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/admin/orders":
			if r.URL.Query().Get("status") == "QUEUED" {
				_, _ = w.Write([]byte(`{"data":{"total":0,"items":[]}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"total":1,"items":[{"order_no":"test","user_id":1,"status":"WAIT_PAY"}]}}`))
			}
		case "/api/ops/stock/reconcile":
			_, _ = w.Write([]byte(`{"data":{"checked":1,"missing":0,"mismatched":0,"items":[]}}`))
		case "/api/activities":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"stock":4}]}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := client{base: server.URL, http: server.Client()}
	v, orderNo, err := c.verify("test-token", 1, 5, 1)
	if err != nil || v["passed"] != true || orderNo != "test" {
		t.Fatalf("verification=%v order=%s err=%v", v, orderNo, err)
	}
}

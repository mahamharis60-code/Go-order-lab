package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	defaultStore = NewStore()
	latencyBins  = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
)

type requestKey struct {
	Method string
	Path   string
	Status string
}

type businessKey struct {
	Event  string
	Result string
}

type bucketKey struct {
	Request requestKey
	LE      string
}

type Store struct {
	mu              sync.RWMutex
	httpRequests    map[requestKey]uint64
	httpDurationSum map[requestKey]float64
	httpDuration    map[bucketKey]uint64
	businessEvents  map[businessKey]uint64
}

func NewStore() *Store {
	return &Store{
		httpRequests:    make(map[requestKey]uint64),
		httpDurationSum: make(map[requestKey]float64),
		httpDuration:    make(map[bucketKey]uint64),
		businessEvents:  make(map[businessKey]uint64),
	}
}

func ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	defaultStore.ObserveHTTPRequest(method, path, status, duration)
}

func IncBusinessEvent(event, result string) {
	defaultStore.IncBusinessEvent(event, result)
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(defaultStore.RenderPrometheus()))
	})
}

func (s *Store) ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	key := requestKey{
		Method: method,
		Path:   path,
		Status: strconv.Itoa(status),
	}
	seconds := duration.Seconds()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.httpRequests[key]++
	s.httpDurationSum[key] += seconds
	for _, bin := range latencyBins {
		if seconds <= bin {
			s.httpDuration[bucketKey{Request: key, LE: formatFloat(bin)}]++
		}
	}
}

func (s *Store) IncBusinessEvent(event, result string) {
	key := businessKey{Event: event, Result: result}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.businessEvents[key]++
}

func (s *Store) RenderPrometheus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder
	b.WriteString("# HELP go_order_http_requests_total Total HTTP requests handled by the service.\n")
	b.WriteString("# TYPE go_order_http_requests_total counter\n")
	for _, key := range sortedRequestKeys(s.httpRequests) {
		fmt.Fprintf(&b,
			"go_order_http_requests_total{method=%s,path=%s,status=%s} %d\n",
			labelValue(key.Method),
			labelValue(key.Path),
			labelValue(key.Status),
			s.httpRequests[key],
		)
	}

	b.WriteString("# HELP go_order_http_request_duration_seconds HTTP request latency histogram.\n")
	b.WriteString("# TYPE go_order_http_request_duration_seconds histogram\n")
	for _, key := range sortedRequestKeys(s.httpRequests) {
		for _, bin := range latencyBins {
			le := formatFloat(bin)
			fmt.Fprintf(&b,
				"go_order_http_request_duration_seconds_bucket{method=%s,path=%s,status=%s,le=%s} %d\n",
				labelValue(key.Method),
				labelValue(key.Path),
				labelValue(key.Status),
				labelValue(le),
				s.httpDuration[bucketKey{Request: key, LE: le}],
			)
		}
		fmt.Fprintf(&b,
			"go_order_http_request_duration_seconds_bucket{method=%s,path=%s,status=%s,le=%s} %d\n",
			labelValue(key.Method),
			labelValue(key.Path),
			labelValue(key.Status),
			labelValue("+Inf"),
			s.httpRequests[key],
		)
		fmt.Fprintf(&b,
			"go_order_http_request_duration_seconds_sum{method=%s,path=%s,status=%s} %s\n",
			labelValue(key.Method),
			labelValue(key.Path),
			labelValue(key.Status),
			formatFloat(s.httpDurationSum[key]),
		)
		fmt.Fprintf(&b,
			"go_order_http_request_duration_seconds_count{method=%s,path=%s,status=%s} %d\n",
			labelValue(key.Method),
			labelValue(key.Path),
			labelValue(key.Status),
			s.httpRequests[key],
		)
	}

	b.WriteString("# HELP go_order_business_events_total Business events emitted by order workflows.\n")
	b.WriteString("# TYPE go_order_business_events_total counter\n")
	for _, key := range sortedBusinessKeys(s.businessEvents) {
		fmt.Fprintf(&b,
			"go_order_business_events_total{event=%s,result=%s} %d\n",
			labelValue(key.Event),
			labelValue(key.Result),
			s.businessEvents[key],
		)
	}

	return b.String()
}

func sortedRequestKeys(values map[requestKey]uint64) []requestKey {
	keys := make([]requestKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Path != keys[j].Path {
			return keys[i].Path < keys[j].Path
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Status < keys[j].Status
	})
	return keys
}

func sortedBusinessKeys(values map[businessKey]uint64) []businessKey {
	keys := make([]businessKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Event != keys[j].Event {
			return keys[i].Event < keys[j].Event
		}
		return keys[i].Result < keys[j].Result
	})
	return keys
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func labelValue(value string) string {
	return `"` + escapeLabel(value) + `"`
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

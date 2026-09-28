package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

type sample struct {
	Status  int
	Message string
	Latency time.Duration
	Failed  bool
}

type report struct {
	Name            string         `json:"name"`
	Workers         int            `json:"request_workers"`
	Requests        int            `json:"requests"`
	ElapsedSeconds  float64        `json:"elapsed_seconds"`
	QPS             float64        `json:"completed_requests_per_second"`
	AcceptedQPS     float64        `json:"accepted_orders_per_second"`
	MeanMS          float64        `json:"mean_ms"`
	P50MS           float64        `json:"p50_ms"`
	P95MS           float64        `json:"p95_ms"`
	P99MS           float64        `json:"p99_ms"`
	MaxMS           float64        `json:"max_ms"`
	SystemErrors    int            `json:"system_errors"`
	SystemErrorRate float64        `json:"system_error_rate"`
	Statuses        map[int]int    `json:"http_status_counts"`
	Messages        map[string]int `json:"response_message_counts"`
	ActivityID      int            `json:"activity_id,omitempty"`
	Verification    map[string]any `json:"verification,omitempty"`
}

type client struct {
	base string
	http *http.Client
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *client) request(ctx context.Context, method, route, token string, body any) (sample, envelope, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return sample{}, envelope{}, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+route, bytes.NewReader(payload))
	if err != nil {
		return sample{}, envelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		kind := "transport_error"
		if os.IsTimeout(err) {
			kind = "request_timeout"
		}
		return sample{Latency: time.Since(start), Failed: true, Message: kind}, envelope{}, err
	}
	defer res.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	var env envelope
	decodeErr := json.Unmarshal(data, &env)
	s := sample{Status: res.StatusCode, Message: env.Message, Latency: time.Since(start), Failed: readErr != nil || decodeErr != nil}
	if s.Failed {
		s.Message = "invalid_or_incomplete_response"
	}
	if readErr != nil {
		return s, env, readErr
	}
	return s, env, decodeErr
}

func (c *client) api(method, route, token string, body any, expected int, target any) error {
	s, env, err := c.request(context.Background(), method, route, token, body)
	if err != nil {
		return err
	}
	if s.Status != expected {
		return fmt.Errorf("%s %s: status=%d message=%s", method, route, s.Status, env.Message)
	}
	if target != nil {
		return json.Unmarshal(env.Data, target)
	}
	return nil
}

// Each request worker issues one request at a time. HTTP transport has its own goroutines.
func runLoad(name string, workers, count int, duration time.Duration, call func(int) sample) report {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]sample, 0)
	next := 0
	start := time.Now()
	deadline := start.Add(duration)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if (duration > 0 && !time.Now().Before(deadline)) || (duration == 0 && next >= count) {
					mu.Unlock()
					return
				}
				index := next
				next++
				mu.Unlock()
				s := call(index)
				mu.Lock()
				results = append(results, s)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return summarize(name, workers, time.Since(start), results)
}

func summarize(name string, workers int, elapsed time.Duration, samples []sample) report {
	r := report{Name: name, Workers: workers, Requests: len(samples), ElapsedSeconds: elapsed.Seconds(), Statuses: map[int]int{}, Messages: map[string]int{}}
	latencies := make([]float64, 0, len(samples))
	for _, s := range samples {
		r.Statuses[s.Status]++
		r.Messages[s.Message]++
		if s.Failed || s.Status >= 500 {
			r.SystemErrors++
		}
		ms := float64(s.Latency) / float64(time.Millisecond)
		latencies = append(latencies, ms)
		r.MeanMS += ms
	}
	if len(samples) == 0 {
		return r
	}
	sort.Float64s(latencies)
	r.MeanMS /= float64(len(samples))
	r.P50MS = percentile(latencies, .50)
	r.P95MS = percentile(latencies, .95)
	r.P99MS = percentile(latencies, .99)
	r.MaxMS = latencies[len(latencies)-1]
	r.SystemErrorRate = float64(r.SystemErrors) / float64(len(samples))
	if elapsed > 0 {
		r.QPS = float64(len(samples)) / elapsed.Seconds()
		r.AcceptedQPS = float64(r.Statuses[202]) / elapsed.Seconds()
	}
	return r
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(math.Ceil(p*float64(len(sorted))))-1]
}

func (c *client) register(prefix string) (string, error) {
	var data struct {
		Token string `json:"token"`
	}
	err := c.api("POST", "/api/auth/register", "", map[string]any{"username": prefix, "password": "pressure-test-only"}, 201, &data)
	if err == nil && data.Token == "" {
		err = errors.New("registration returned empty token")
	}
	return data.Token, err
}

func (c *client) activity(admin, suffix string, stock int) (int, error) {
	var product struct {
		ID int `json:"id"`
	}
	err := c.api("POST", "/api/products", admin, map[string]any{"name": "pressure-" + suffix, "price": 19900, "stock": stock}, 201, &product)
	if err != nil {
		return 0, err
	}
	var activity struct {
		ID int `json:"id"`
	}
	err = c.api("POST", "/api/activities", admin, map[string]any{"product_id": product.ID, "name": "pressure-" + suffix, "price": 18900, "stock": stock, "duration_seconds": 3600}, 201, &activity)
	return activity.ID, err
}

type orderList struct {
	Total int `json:"total"`
	Items []struct {
		OrderNo string `json:"order_no"`
		UserID  int    `json:"user_id"`
		Status  string `json:"status"`
	} `json:"items"`
}

// Verification reads persisted orders and both stock stores after async processing settles.
func (c *client) verify(admin string, id, initial, accepted int) (map[string]any, string, error) {
	verificationStart := time.Now()
	var orders orderList
	deadline := time.Now().Add(20 * time.Second)
	for {
		if err := c.api("GET", fmt.Sprintf("/api/admin/orders?activity_id=%d&limit=100", id), admin, nil, 200, &orders); err != nil {
			return nil, "", err
		}
		var queued orderList
		if err := c.api("GET", fmt.Sprintf("/api/admin/orders?activity_id=%d&status=QUEUED&limit=1", id), admin, nil, 200, &queued); err != nil {
			return nil, "", err
		}
		if queued.Total == 0 {
			break
		}
		if time.Now().After(deadline) {
			return nil, "", errors.New("orders did not leave QUEUED within 20s")
		}
		time.Sleep(200 * time.Millisecond)
	}
	var stock struct {
		Checked    int `json:"checked"`
		Missing    int `json:"missing"`
		Mismatched int `json:"mismatched"`
		Items      []struct {
			MySQL  int  `json:"mysql_stock"`
			Redis  int  `json:"redis_stock"`
			Exists bool `json:"redis_exists"`
		} `json:"items"`
		Failed int `json:"failed"`
	}
	if err := c.api("POST", "/api/ops/stock/reconcile", admin, map[string]any{"activity_id": id, "repair": false}, 200, &stock); err != nil {
		return nil, "", err
	}
	var activities []struct {
		ID    int `json:"id"`
		Stock int `json:"stock"`
	}
	if err := c.api("GET", "/api/activities", "", nil, 200, &activities); err != nil {
		return nil, "", err
	}
	mysqlStock := -1
	for _, activity := range activities {
		if activity.ID == id {
			mysqlStock = activity.Stock
			break
		}
	}
	seen := map[int]bool{}
	duplicate := false
	for _, o := range orders.Items {
		if seen[o.UserID] {
			duplicate = true
		}
		seen[o.UserID] = true
		if o.Status != "WAIT_PAY" {
			return nil, "", fmt.Errorf("unexpected order state %s", o.Status)
		}
	}
	// Reconcile returns only mismatched/missing items, not every checked activity.
	redisMatches := stock.Checked == 1 && stock.Missing == 0 && stock.Mismatched == 0 && len(stock.Items) == 0
	valid := orders.Total == accepted && mysqlStock == initial-accepted && redisMatches && !duplicate && orders.Total == len(orders.Items)
	v := map[string]any{"passed": valid, "initial_stock": initial, "accepted_requests": accepted, "persisted_orders": orders.Total, "mysql_stock": mysqlStock, "redis_matches_mysql": redisMatches, "redis_evidence": "read-only reconcile counters", "duplicate_user_order": duplicate, "all_orders_wait_pay": true}
	v["async_verification_seconds"] = time.Since(verificationStart).Seconds()
	if !valid {
		return v, "", fmt.Errorf("verification failed: %v", v)
	}
	if len(orders.Items) == 0 {
		return v, "", errors.New("no accepted order to query")
	}
	return v, orders.Items[0].OrderNo, nil
}

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	base := flag.String("base-url", "http://127.0.0.1:8090", "HTTP API base URL")
	workers := flag.Int("concurrency", 20, "fixed request worker goroutines")
	users := flag.Int("users", 200, "distinct users for stock competition (1..5000)")
	stock := flag.Int("stock", 30, "competition inventory (1..100, not greater than users)")
	duplicates := flag.Int("duplicate-requests", 100, "same-user request count")
	writes := flag.Int("write-requests", 1000, "unique valid order requests; zero skips this workload")
	duration := flag.Duration("duration", 15*time.Second, "sustained order-query duration")
	out := flag.String("out", "reports/pressure-go.json", "report path")
	flag.Parse()
	if *workers < 1 || *users < 1 || *users > 5000 || *stock < 1 || *stock > 100 || *stock > *users || *duplicates < 1 || *writes < 0 || *duration <= 0 {
		return errors.New("invalid load configuration")
	}
	password := os.Getenv("ORDER_ADMIN_PASSWORD")
	if password == "" {
		return errors.New("set ORDER_ADMIN_PASSWORD; credentials are never written to reports")
	}
	username := os.Getenv("ORDER_ADMIN_USERNAME")
	if username == "" {
		username = "admin"
	}
	transport := &http.Transport{MaxIdleConns: *workers * 2, MaxIdleConnsPerHost: *workers, MaxConnsPerHost: *workers, IdleConnTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	c := &client{base: *base, http: &http.Client{Transport: transport, Timeout: 10 * time.Second}}
	var login struct {
		Token string `json:"token"`
	}
	if err := c.api("POST", "/api/auth/login", "", map[string]string{"username": username, "password": password}, 200, &login); err != nil {
		return err
	}
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tokens := make([]string, *users)
	// Registration setup is outside all measured windows to exclude bcrypt work.
	for i := range tokens {
		token, err := c.register(fmt.Sprintf("p%s_%d", stamp, i))
		if err != nil {
			return err
		}
		tokens[i] = token
	}
	duplicateID, err := c.activity(login.Token, stamp+"-duplicate", 5)
	if err != nil {
		return err
	}
	stockID, err := c.activity(login.Token, stamp+"-stock", *stock)
	if err != nil {
		return err
	}
	post := func(id int, token, requestID string) sample {
		s, _, _ := c.request(context.Background(), "POST", "/api/orders", token, map[string]any{"activity_id": id, "request_id": requestID})
		return s
	}
	duplicate := runLoad("same_user_duplicate", *workers, *duplicates, 0, func(i int) sample { return post(duplicateID, tokens[0], fmt.Sprintf("d%s-%d", stamp, i)) })
	duplicate.ActivityID = duplicateID
	stockReport := runLoad("multi_user_stock", *workers, *users, 0, func(i int) sample { return post(stockID, tokens[i], fmt.Sprintf("s%s-%d", stamp, i)) })
	stockReport.ActivityID = stockID
	var verificationErrors []string
	v, orderNo, err := c.verify(login.Token, duplicateID, 5, duplicate.Statuses[202])
	duplicate.Verification = v
	if err != nil {
		verificationErrors = append(verificationErrors, err.Error())
	}
	v, _, err = c.verify(login.Token, stockID, *stock, stockReport.Statuses[202])
	stockReport.Verification = v
	if err != nil {
		verificationErrors = append(verificationErrors, err.Error())
	}
	if duplicate.Statuses[202] != 1 {
		verificationErrors = append(verificationErrors, "same-user accepted count must equal 1")
	}
	if stockReport.Statuses[202] != *stock {
		verificationErrors = append(verificationErrors, "multi-user accepted count must equal configured stock; inspect rate limiting/errors")
	}
	reports := []report{duplicate, stockReport}
	if *writes > 0 {
		batchSize := min(*users, 100)
		activityIDs := make([]int, (*writes+batchSize-1)/batchSize)
		for batch := range activityIDs {
			id, createErr := c.activity(login.Token, fmt.Sprintf("%s-write-%d", stamp, batch), min(batchSize, *writes-batch*batchSize))
			if createErr != nil {
				return createErr
			}
			activityIDs[batch] = id
		}
		writeReport := runLoad("valid_order_writes", *workers, *writes, 0, func(i int) sample {
			return post(activityIDs[i/batchSize], tokens[i%batchSize], fmt.Sprintf("w%s-%d", stamp, i))
		})
		writeReport.Verification = map[string]any{"activity_ids": activityIDs, "expected_orders": *writes}
		passed := writeReport.Statuses[202] == *writes
		settleStart := time.Now()
		for batch, id := range activityIDs {
			expected := min(batchSize, *writes-batch*batchSize)
			_, _, checkErr := c.verify(login.Token, id, expected, expected)
			if checkErr != nil {
				passed = false
				verificationErrors = append(verificationErrors, checkErr.Error())
			}
		}
		writeReport.Verification["passed"] = passed
		writeReport.Verification["async_verification_seconds"] = time.Since(settleStart).Seconds()
		if !passed {
			verificationErrors = append(verificationErrors, "valid write workload did not persist every expected order")
		}
		reports = append(reports, writeReport)
	}
	if orderNo != "" {
		read := func(int) sample {
			s, _, _ := c.request(context.Background(), "GET", "/api/orders/"+orderNo, tokens[0], nil)
			return s
		}
		_ = runLoad("warmup", *workers, *workers, 0, read)
		reports = append(reports, runLoad("sustained_order_query", *workers, 0, *duration, read))
	}
	for _, r := range reports {
		if r.SystemErrors > 0 {
			verificationErrors = append(verificationErrors, r.Name+": system errors detected")
		}
		if r.Name == "sustained_order_query" && (r.Requests == 0 || r.Statuses[200] != r.Requests) {
			verificationErrors = append(verificationErrors, "query responses must all be 200")
		}
	}
	output := map[string]any{"started_at": stamp, "recorded_at": time.Now().UTC(), "base_url": *base, "client_go_version": runtime.Version(), "client_os": runtime.GOOS, "client_arch": runtime.GOARCH, "client_logical_cpus": runtime.NumCPU(), "load_model": "closed-loop fixed worker goroutines; no think time; setup and warmup excluded", "percentile_method": "nearest rank; full response-body latency", "reports": reports, "verification_errors": verificationErrors, "passed": len(verificationErrors) == 0}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, data, 0644); err != nil {
		return err
	}
	fmt.Println(string(data))
	if len(verificationErrors) != 0 {
		return fmt.Errorf("load verification failed; see %s", *out)
	}
	return nil
}

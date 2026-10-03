package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type httpMetric struct {
	Count    uint64
	Duration float64
}
type Registry struct {
	mu             sync.Mutex
	http           map[string]httpMetric
	recurring      map[string]uint64
	workerFailures uint64
}

func New() *Registry { return &Registry{http: map[string]httpMetric{}, recurring: map[string]uint64{}} }
func (r *Registry) ObserveHTTP(method, path string, status int, duration time.Duration) {
	key := method + "\x00" + path + "\x00" + strconv.Itoa(status)
	r.mu.Lock()
	item := r.http[key]
	item.Count++
	item.Duration += duration.Seconds()
	r.http[key] = item
	r.mu.Unlock()
}
func (r *Registry) Recurring(status string, count int) {
	if count <= 0 {
		return
	}
	r.mu.Lock()
	r.recurring[status] += uint64(count)
	r.mu.Unlock()
}
func (r *Registry) WorkerFailure() { r.mu.Lock(); r.workerFailures++; r.mu.Unlock() }
func (r *Registry) Handler(activeSessions func() int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		r.mu.Lock()
		httpValues := make(map[string]httpMetric, len(r.http))
		for k, v := range r.http {
			httpValues[k] = v
		}
		recurring := make(map[string]uint64, len(r.recurring))
		for k, v := range r.recurring {
			recurring[k] = v
		}
		failures := r.workerFailures
		r.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		keys := make([]string, 0, len(httpValues))
		for k := range httpValues {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(w, "# TYPE finance_http_requests_total counter")
		fmt.Fprintln(w, "# TYPE finance_http_request_duration_seconds summary")
		for _, key := range keys {
			parts := strings.Split(key, "\x00")
			item := httpValues[key]
			labels := fmt.Sprintf(`method=%q,path=%q,status=%q`, parts[0], parts[1], parts[2])
			fmt.Fprintf(w, "finance_http_requests_total{%s} %d\n", labels, item.Count)
			fmt.Fprintf(w, "finance_http_request_duration_seconds_sum{%s} %f\n", labels, item.Duration)
			fmt.Fprintf(w, "finance_http_request_duration_seconds_count{%s} %d\n", labels, item.Count)
		}
		fmt.Fprintln(w, "# TYPE finance_active_sessions gauge")
		fmt.Fprintf(w, "finance_active_sessions %d\n", activeSessions())
		fmt.Fprintln(w, "# TYPE finance_recurring_executions_total counter")
		statuses := make([]string, 0, len(recurring))
		for status := range recurring {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			fmt.Fprintf(w, "finance_recurring_executions_total{status=%q} %d\n", status, recurring[status])
		}
		fmt.Fprintln(w, "# TYPE finance_worker_failures_total counter")
		fmt.Fprintf(w, "finance_worker_failures_total %d\n", failures)
	})
}

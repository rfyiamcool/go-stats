package stats

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	nilFunc           = func() {}
	defaultConstLabel = make(map[string]string, 10)
)

var (
	// metric:http_request_duration_seconds
	HttpReqDuration *prometheus.HistogramVec

	// metric:http_request_total
	HttpReqTotal *prometheus.CounterVec

	// metric:http_response_bytes
	HttpResponseBytes *prometheus.HistogramVec

	// metric:http_request_bytes
	HttpReqBytes *prometheus.HistogramVec

	// metric: database_duration
	DatabaseDuration *prometheus.HistogramVec

	// metric: func_duration
	FuncDuration *prometheus.HistogramVec
)

func init() {
	initMetrics()
	register()
}

func register() {
	prometheus.MustRegister(
		HttpReqDuration,
		HttpReqTotal,
		HttpReqBytes,
		HttpResponseBytes,
		DatabaseDuration,
		FuncDuration,
	)
}

func unregister() {
	prometheus.Unregister(HttpReqDuration)
	prometheus.Unregister(HttpReqTotal)
	prometheus.Unregister(HttpReqBytes)
	prometheus.Unregister(HttpResponseBytes)
	prometheus.Unregister(DatabaseDuration)
	prometheus.Unregister(FuncDuration)
}

func initMetrics() {
	// request time cost
	HttpReqDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "The HTTP request latencies in seconds.",
		// ConstLabels: defaultConstLabel,
		Buckets: nil,
	}, []string{"method", "path"})

	// request qps
	HttpReqTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_total",
		Help: "Total number of HTTP requests made.",
	}, []string{"method", "path", "status"})

	// request bytes
	HttpReqBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_bytes",
		Help:    "The HTTP request sizes in bytes.",
		Buckets: nil,
	}, []string{"method", "path"})

	// response bytes
	HttpResponseBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_response_bytes",
		Help:    "Response Bytes Size of Each Request.",
		Buckets: nil,
	}, []string{"method", "path"})

	// db time cost
	DatabaseDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "database_duration_seconds",
		Help:    "database request latencies in seconds.",
		Buckets: nil,
	}, []string{"dao", "filter", "args"})

	// custom func time cost
	FuncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "func_duration_seconds",
		Help:    "func latencies in seconds.",
		Buckets: nil,
	}, []string{"func", "args"})
}

// todo
// func SetConstLabels(m map[string]string) {
// 	if m == nil || len(m) == 0 {
// 		return
// 	}

// todo
// 	defaultConstLabel = m
// 	initMetrics()
// }

func extractRequestPath(path string) string {
	itemList := strings.Split(path, "/")
	if len(path) >= 4 {
		return strings.Join(itemList[0:3], "/")
	}
	return path
}

func parseRequestUrl(c *gin.Context) string {
	url := c.Request.URL.Path
	for _, p := range c.Params {
		key := ":" + p.Key
		url = strings.Replace(url, p.Value, key, 1)
	}
	return url
}

// Metric metric middleware
func GinMetricMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := timeSince(start)
		path := parseRequestUrl(c)
		reqsz := getRequestSize(c.Request)

		hrtlabels := prometheus.Labels{
			"method": c.Request.Method,
			"path":   path,
			"status": strconv.Itoa(c.Writer.Status()),
		}
		HttpReqTotal.With(hrtlabels).Inc()

		// same labels
		labels := prometheus.Labels{
			"method": c.Request.Method,
			"path":   path,
		}
		HttpReqDuration.With(labels).Observe(duration)
		HttpResponseBytes.With(labels).Observe(float64(c.Writer.Size()))
		HttpReqBytes.With(labels).Observe(reqsz)
	}
}

func SetHttpReqStats(c *gin.Context, path string, start time.Time) {
	HttpReqTotal.With(prometheus.Labels{
		"method": c.Request.Method,
		"path":   path,
		"status": strconv.Itoa(c.Writer.Status()),
	}).Inc()

	duration := timeSince(start)
	HttpReqDuration.With(prometheus.Labels{
		"method": c.Request.Method,
		"path":   path,
	}).Observe(duration)
}

func SetHttpReqStatsWrap(c *gin.Context, path string) func() {
	start := time.Now()
	return func() {
		SetHttpReqStats(c, path, start)
	}
}

// usage:
// done := NewDatabaseDurationStats(...)
// do...
// done()
func NewDatabaseDurationStats(dao string, filter string, args []interface{}) func() {
	return SetDatabaseDurationStatsWrap(dao, filter, args)
}

// usage:
// start := time.Now()
// do...
// SetDatabaseDurationStats(..., start)
func SetDatabaseDurationStats(dao string, filter string, args []interface{}, start time.Time) {
	duration := timeSince(start)
	DatabaseDuration.With(prometheus.Labels{
		"dao":    strings.ToLower(dao),
		"filter": strings.ToLower(filter),
		"args":   formatArgs(args),
	}).Observe(duration)
}

// usage:
// defer SetDatabaseDurationStatsWrap(dao, filter, ...)()
func SetDatabaseDurationStatsWrap(dao string, filter string, args []interface{}) func() {
	if dao == "" {
		return nilFunc
	}

	start := time.Now()
	return func() {
		SetDatabaseDurationStats(dao, filter, args, start)
	}
}

func NewFuncDurationStats(fn string, args []interface{}) func() {
	return SetFuncDurationStatsWrap(fn, args)
}

func SetFuncDurationStats(fn string, args []interface{}, start time.Time) {
	duration := timeSince(start)
	FuncDuration.With(prometheus.Labels{
		"func": fn,
		"args": formatArgs(args),
	}).Observe(duration)
}

func SetFuncDurationStatsWrap(fn string, args []interface{}) func() {
	if fn == "" {
		return nilFunc
	}

	start := time.Now()
	return func() {
		SetFuncDurationStats(fn, args, start)
	}
}

func timeSince(start time.Time) float64 {
	minv := float64(0.001) // 1ms
	val := float64(time.Since(start)) / float64(time.Second)
	if val < minv {
		return minv
	}
	return val
}

func formatArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	ret := ""
	for _, val := range args {
		if ret == "" {
			ret = fmt.Sprintf("%v", val)
			continue
		}

		ret = fmt.Sprintf("%s_%v", ret, val)
	}
	return ret
}

func getRequestSize(r *http.Request) float64 {
	size := 0
	if r.URL != nil {
		size = len(r.URL.Path)
	}

	size += len(r.Method)
	size += len(r.Proto)
	for name, values := range r.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(r.Host)

	if r.ContentLength != -1 {
		size += int(r.ContentLength)
	}
	return float64(size)
}

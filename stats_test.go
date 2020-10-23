package stats

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestFormatArgs(t *testing.T) {
	args := []interface{}{"name", "addr", 88}
	v := formatArgs(args)
	assert.Equal(t, v, "name_addr_88")
}

func TestTimeSince(t *testing.T) {
	start := time.Now()
	ds := timeSince(start)
	assert.Equal(t, ds, 0.001) // min val
}

func TestSimple(t *testing.T) {
	defer SetFuncDurationStatsWrap("test_func_1", nil)()
	start := time.Now()
	done := NewFuncDurationStats("test_func_2", nil)
	time.Sleep(1 * time.Second)
	SetFuncDurationStats("test_func_3", nil, start)
	done()
}

// test gin
func TestGin(t *testing.T) {
	router := gin.Default()

	// add gin metric middleware
	router.Use(GinMetricMiddleware())

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	go router.Run(":1888")

	time.AfterFunc(10*time.Second, unregister)
	time.Sleep(20 * time.Second)
}

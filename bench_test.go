package fire

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
)

const benchListItems = 20

var benchStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-coal?maxPoolSize=100", xo.Crash)

var benchThrottle = 100

func BenchmarkList(b *testing.B) {
	b.Run("00X", func(b *testing.B) {
		listBenchmark(b, benchStore, 0)
	})

	b.Run("01X", func(b *testing.B) {
		listBenchmark(b, benchStore, 1)
	})

	b.Run("10X", func(b *testing.B) {
		listBenchmark(b, benchStore, 10)
	})

	b.Run("50X", func(b *testing.B) {
		listBenchmark(b, benchStore, 50)
	})

	b.Run("100X", func(b *testing.B) {
		listBenchmark(b, benchStore, 100)
	})
}

func BenchmarkFind(b *testing.B) {
	b.Run("00X", func(b *testing.B) {
		findBenchmark(b, benchStore, 0)
	})

	b.Run("01X", func(b *testing.B) {
		findBenchmark(b, benchStore, 1)
	})

	b.Run("10X", func(b *testing.B) {
		findBenchmark(b, benchStore, 10)
	})

	b.Run("50X", func(b *testing.B) {
		findBenchmark(b, benchStore, 50)
	})

	b.Run("100X", func(b *testing.B) {
		findBenchmark(b, benchStore, 100)
	})
}

func BenchmarkCreate(b *testing.B) {
	b.Run("00X", func(b *testing.B) {
		createBenchmark(b, benchStore, 0)
	})

	b.Run("01X", func(b *testing.B) {
		createBenchmark(b, benchStore, 1)
	})

	b.Run("10X", func(b *testing.B) {
		createBenchmark(b, benchStore, 10)
	})

	b.Run("50X", func(b *testing.B) {
		createBenchmark(b, benchStore, 50)
	})

	b.Run("100X", func(b *testing.B) {
		createBenchmark(b, benchStore, 100)
	})
}

func listBenchmark(b *testing.B, store *coal.Store, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	group := tester.Assign("", &Controller{
		Model: &postModel{},
	}, &Controller{
		Model: &commentModel{},
	}, &Controller{
		Model: &selectionModel{},
	}, &Controller{
		Model: &noteModel{},
	})

	tester.Handler = serve.Compose(
		serve.Throttle(benchThrottle),
		tester.Handler,
	)

	group.reporter = func(err error) {}

	for i := 0; i < benchListItems; i++ {
		tester.Insert(&postModel{
			Title:    "Hello World!",
			TextBody: strings.Repeat("X", 100),
		})
	}

	parallelBenchmark(b, parallelism, func() bool {
		res := serve.Record(tester.Handler, "GET", "/posts", nil, "")
		return res.Code == http.StatusOK
	})
}

func findBenchmark(b *testing.B, store *coal.Store, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	group := tester.Assign("", &Controller{
		Model: &postModel{},
	}, &Controller{
		Model: &commentModel{},
	}, &Controller{
		Model: &selectionModel{},
	}, &Controller{
		Model: &noteModel{},
	})

	tester.Handler = serve.Compose(
		serve.Throttle(benchThrottle),
		tester.Handler,
	)

	group.reporter = func(err error) {}

	id := tester.Insert(&postModel{
		Title:    "Hello World!",
		TextBody: strings.Repeat("X", 100),
	}).ID()

	parallelBenchmark(b, parallelism, func() bool {
		res := serve.Record(tester.Handler, "GET", "/posts/"+id.Hex(), nil, "")
		return res.Code == http.StatusOK
	})
}

func createBenchmark(b *testing.B, store *coal.Store, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	group := tester.Assign("", &Controller{
		Model: &postModel{},
	}, &Controller{
		Model: &commentModel{},
	}, &Controller{
		Model: &selectionModel{},
	}, &Controller{
		Model: &noteModel{},
	})

	tester.Handler = serve.Compose(
		serve.Throttle(benchThrottle),
		tester.Handler,
	)

	group.reporter = func(err error) {}

	headers := map[string]string{
		"Content-Type": jsonapi.MediaType,
	}

	parallelBenchmark(b, parallelism, func() bool {
		res := serve.Record(tester.Handler, "POST", "/posts", headers, `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "Post 1",
					"text-body": "`+strings.Repeat("X", 100)+`"
				}
			}
		}`)
		return res.Code == http.StatusCreated
	})
}

func parallelBenchmark(b *testing.B, parallelism int, fn func() bool) {
	if parallelism != 0 {
		b.SetParallelism(parallelism)
	}

	b.ReportAllocs()
	b.ResetTimer()

	now := time.Now()

	var errs int64

	if parallelism > 0 {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if !fn() {
					atomic.AddInt64(&errs, 1)
				}
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			if !fn() {
				errs++
			}
		}
	}

	b.StopTimer()

	nsPerOp := float64(time.Since(now)) / float64(b.N)
	opsPerS := float64(time.Second) / nsPerOp
	b.ReportMetric(opsPerS, "ops/s")

	b.ReportMetric(float64(errs), "errors")
}

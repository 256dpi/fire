package fire

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/256dpi/serve"

	"github.com/256dpi/fire/coal"
)

var textBody = strings.Repeat("X", 100)

func BenchmarkList(b *testing.B) {
	store := mongoStore

	b.Run("00X", func(b *testing.B) {
		listBenchmark(b, store, 20, 0)
	})

	b.Run("01X", func(b *testing.B) {
		listBenchmark(b, store, 20, 1)
	})

	b.Run("10X", func(b *testing.B) {
		listBenchmark(b, store, 20, 10)
	})

	b.Run("50X", func(b *testing.B) {
		listBenchmark(b, store, 20, 50)
	})

	b.Run("100X", func(b *testing.B) {
		listBenchmark(b, store, 20, 100)
	})
}

func BenchmarkFind(b *testing.B) {
	store := mongoStore

	b.Run("00X", func(b *testing.B) {
		findBenchmark(b, store, 0)
	})

	b.Run("01X", func(b *testing.B) {
		findBenchmark(b, store, 1)
	})

	b.Run("10X", func(b *testing.B) {
		findBenchmark(b, store, 10)
	})

	b.Run("50X", func(b *testing.B) {
		findBenchmark(b, store, 50)
	})

	b.Run("100X", func(b *testing.B) {
		findBenchmark(b, store, 100)
	})
}

func BenchmarkCreate(b *testing.B) {
	store := mongoStore

	b.Run("00X", func(b *testing.B) {
		createBenchmark(b, store, 0)
	})

	b.Run("01X", func(b *testing.B) {
		createBenchmark(b, store, 1)
	})

	b.Run("10X", func(b *testing.B) {
		createBenchmark(b, store, 10)
	})

	b.Run("50X", func(b *testing.B) {
		createBenchmark(b, store, 50)
	})

	b.Run("100X", func(b *testing.B) {
		createBenchmark(b, store, 100)
	})
}

func listBenchmark(b *testing.B, store *coal.Store, items, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	tester.Handler = serve.Compose(
		serve.Throttle(100),
		tester.Handler,
	)

	for i := 0; i < items; i++ {
		tester.Insert(&postModel{
			Title:    "Hello World!",
			TextBody: textBody,
		})
	}

	parallelBenchmark(b, parallelism, func() {
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			if r.Code != http.StatusOK {
				panic("not ok")
			}
		})
	})
}

func findBenchmark(b *testing.B, store *coal.Store, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	tester.Handler = serve.Compose(
		serve.Throttle(100),
		tester.Handler,
	)

	id := tester.Insert(&postModel{
		Title:    "Hello World!",
		TextBody: textBody,
	}).ID()

	parallelBenchmark(b, parallelism, func() {
		tester.Request("GET", "posts/"+id.Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			if r.Code != http.StatusOK {
				panic("not ok")
			}
		})
	})
}

func createBenchmark(b *testing.B, store *coal.Store, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	tester.Handler = serve.Compose(
		serve.Throttle(100),
		tester.Handler,
	)

	parallelBenchmark(b, parallelism, func() {
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			if r.Code != http.StatusCreated {
				panic("not ok")
			}
		})
	})
}

func parallelBenchmark(b *testing.B, parallelism int, fn func()) {
	if parallelism != 0 {
		b.SetParallelism(parallelism)
	}

	b.ReportAllocs()
	b.ResetTimer()

	now := time.Now()

	if parallelism != 0 {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				fn()
			}
		})
	} else {
		for i := 0; i < b.N; i++ {
			fn()
		}
	}

	b.StopTimer()

	nsPerOp := float64(time.Since(now)) / float64(b.N)
	opsPerS := float64(time.Second) / nsPerOp
	b.ReportMetric(opsPerS, "ops/s")
}

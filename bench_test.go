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

	b.Run("1X", func(b *testing.B) {
		listBenchmark(b, store, false, 20, 0)
	})

	b.Run("10X", func(b *testing.B) {
		listBenchmark(b, store, false, 20, 10)
	})

	b.Run("50X", func(b *testing.B) {
		listBenchmark(b, store, false, 20, 50)
	})

	b.Run("10X-TX", func(b *testing.B) {
		listBenchmark(b, store, true, 20, 10)
	})

	// connection pool under pressure
	b.Run("50X-TX", func(b *testing.B) {
		listBenchmark(b, store, true, 20, 50)
	})
}

func BenchmarkFind(b *testing.B) {
	store := mongoStore

	b.Run("1X", func(b *testing.B) {
		findBenchmark(b, store, false, 0)
	})

	b.Run("10X", func(b *testing.B) {
		findBenchmark(b, store, false, 10)
	})

	b.Run("50X", func(b *testing.B) {
		findBenchmark(b, store, false, 50)
	})

	b.Run("10X-TX", func(b *testing.B) {
		findBenchmark(b, store, true, 10)
	})

	// connection pool under pressure
	b.Run("50X-TX", func(b *testing.B) {
		findBenchmark(b, store, true, 50)
	})
}

func listBenchmark(b *testing.B, store *coal.Store, transactions bool, items, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	tester.Assign("", &Controller{
		Model:           &postModel{},
		Store:           tester.Store,
		UseTransactions: transactions,
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

	tester.Handler = serve.Compose(Throttle(100), tester.Handler)

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

func findBenchmark(b *testing.B, store *coal.Store, transactions bool, parallelism int) {
	tester := NewTester(store, modelList...)
	tester.Clean()

	tester.Assign("", &Controller{
		Model:           &postModel{},
		Store:           tester.Store,
		UseTransactions: transactions,
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

	tester.Handler = serve.Compose(Throttle(100), tester.Handler)

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

func parallelBenchmark(b *testing.B, parallelism int, fn func()) {
	if parallelism > 0 {
		b.SetParallelism(parallelism)
	}

	b.ReportAllocs()
	b.ResetTimer()

	now := time.Now()

	if parallelism > 0 {
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

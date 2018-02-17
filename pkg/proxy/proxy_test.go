package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	// "github.com/stretchr/testify/require"

	"github.com/altipla-consulting/baster/pkg/config"
)

func BenchmarkSimpleGet(b *testing.B) {
	log.SetLevel(log.ErrorLevel)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") }))
	defer ts.Close()

	h := Handler(config.Domain{
		Name:     "test",
		Hostname: "test.example.com",
		Service:  ts.URL,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = h
		fmt.Sprintf("hello")
	}
}

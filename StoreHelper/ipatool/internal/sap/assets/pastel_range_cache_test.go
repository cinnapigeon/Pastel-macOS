package assets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCachedRangeAndFallback(t *testing.T) {
	content := []byte("abcdefghij")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Range") != "bytes=0-1" {
			t.Errorf("unexpected range %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-1/%d", len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[:2])
	}))
	defer server.Close()
	file := &remoteFile{ctx: context.Background(), client: server.Client(), url: server.URL, size: int64(len(content)), cacheStart: 3, cacheData: content[3:8]}
	buf := make([]byte, 3)
	if n, err := file.ReadAt(buf, 4); err != nil || n != 3 || string(buf) != "efg" {
		t.Fatalf("cached read: %q, %d, %v", buf, n, err)
	}
	if requests != 0 {
		t.Fatalf("cached read made %d requests", requests)
	}
	buf = make([]byte, 2)
	if n, err := file.ReadAt(buf, 0); err != nil || n != 2 || string(buf) != "ab" {
		t.Fatalf("remote read: %q, %d, %v", buf, n, err)
	}
	if requests != 1 {
		t.Fatalf("remote read made %d requests", requests)
	}
}

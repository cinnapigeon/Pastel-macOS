package assets

import (
	"fmt"
	"io"
	"os"
)

// Report only fixed component names and byte counts; never include account data.
func assetProgress(name string, done, total int64) {
	if os.Getenv("PASTEL_ASSET_EVENTS") == "1" {
		fmt.Fprintf(os.Stderr, "@@PASTEL:asset=%s|%d|%d\n", name, done, total)
	}
}

type assetProgressWriter struct {
	writer  io.Writer
	name    string
	total   int64
	written int64
	last    int64
}

func (w *assetProgressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.written += int64(n)
	percent := int64(0)
	if w.total > 0 {
		percent = 100 * w.written / w.total
	}
	if percent > w.last || w.written == w.total {
		w.last = percent
		assetProgress(w.name, w.written, w.total)
	}
	return n, err
}

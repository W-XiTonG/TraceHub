package gateway

import (
	"io"
	"net/http"
)

// flushAfterWrite 将上游分块写出并在每次成功写入后 Flush，避免中间层缓冲导致 SSE 体感卡顿。
type flushAfterWrite struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushAfterWrite) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if n > 0 {
		fw.f.Flush()
	}
	return n, err
}

// WriterFlushWrap 在 ResponseWriter 支持 Flush 时，返回「每次 Write 成功后立即 Flush」的包装器，
// 用于 SSE、音频流、大文件分块等，避免 gin/反向代理缓冲导致首包与增量延迟。
func WriterFlushWrap(w http.ResponseWriter) io.Writer {
	if f, ok := w.(http.Flusher); ok {
		return &flushAfterWrite{w: w, f: f}
	}
	return w
}

// copyStreamFlushing 将 src 复制到 HTTP 响应；若 dst 支持 Flusher，则在每次成功 Write 后 Flush。
func copyStreamFlushing(dst http.ResponseWriter, src io.Reader) (int64, error) {
	return io.Copy(WriterFlushWrap(dst), src)
}

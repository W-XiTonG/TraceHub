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

// copyStreamFlushing 将 src 复制到 HTTP 响应；若 dst 支持 Flusher，则在每次成功 Write 后 Flush。
func copyStreamFlushing(dst http.ResponseWriter, src io.Reader) (int64, error) {
	if f, ok := dst.(http.Flusher); ok {
		return io.Copy(&flushAfterWrite{w: dst, f: f}, src)
	}
	return io.Copy(dst, src)
}

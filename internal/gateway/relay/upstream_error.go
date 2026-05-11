package relay

// UpstreamJSONError 表示上游返回了非 2xx 且 body 宜原样透传给客户端（通常为 JSON）。
type UpstreamJSONError struct {
	StatusCode int
	Body       []byte
}

func (e *UpstreamJSONError) Error() string {
	return "upstream http error"
}

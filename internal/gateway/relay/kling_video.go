package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"tracehub/internal/pkg/klingvideo"
)

// ForwardKlingVideoCreate POST 可灵 text2video 或 image2video。
func ForwardKlingVideoCreate(ctx context.Context, baseURL, apiKey string, openAIBody []byte) (*http.Response, klingvideo.CreateKind, error) {
	kind, body, err := klingvideo.PrepareKlingVideoCreateBody(openAIBody)
	if err != nil {
		return nil, "", err
	}
	path := klingvideo.PathText2Video
	if kind == klingvideo.CreateImage2Video {
		path = klingvideo.PathImage2Video
	}
	resp, err := forwardKlingHTTP(ctx, baseURL, apiKey, http.MethodPost, path, body, "", nil)
	return resp, kind, err
}

// ForwardKlingVideoTaskHTTP 查询/删除可灵视频任务。
func ForwardKlingVideoTaskHTTP(ctx context.Context, baseURL, apiKey, method, taskID string, kind klingvideo.CreateKind, rawQuery string, passthrough http.Header) (*http.Response, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("empty task id")
	}
	if method == http.MethodDelete {
		// 可灵文档未统一 DELETE；返回 405 式错误由上游处理，仍尝试 GET 路径语义
	}
	path := kind.QueryPath(taskID)
	if kind == "" {
		path = klingvideo.CreateText2Video.QueryPath(taskID)
	}
	resp, err := forwardKlingHTTP(ctx, baseURL, apiKey, method, path, nil, rawQuery, passthrough)
	if err != nil {
		return nil, err
	}
	if kind == "" && resp != nil && resp.StatusCode == http.StatusNotFound {
		DrainBody(resp.Body)
		alt := klingvideo.CreateImage2Video.QueryPath(taskID)
		return forwardKlingHTTP(ctx, baseURL, apiKey, method, alt, nil, rawQuery, passthrough)
	}
	return resp, nil
}

// ForwardKlingVideoContent 任务完成后从 video_url 拉取 MP4。
func ForwardKlingVideoContent(ctx context.Context, baseURL, apiKey, taskID string, kind klingvideo.CreateKind, passthrough http.Header) (*http.Response, error) {
	taskResp, err := ForwardKlingVideoTaskHTTP(ctx, baseURL, apiKey, http.MethodGet, taskID, kind, "", passthrough)
	if err != nil {
		return nil, err
	}
	defer DrainBody(taskResp.Body)
	taskBody, _ := io.ReadAll(taskResp.Body)
	if taskResp.StatusCode >= 400 {
		return &http.Response{
			StatusCode: taskResp.StatusCode,
			Header:     taskResp.Header.Clone(),
			Body:       io.NopCloser(strings.NewReader(string(taskBody))),
		}, nil
	}
	videoURL, ok := klingvideo.ExtractVideoURLFromTaskBody(taskBody)
	if !ok || videoURL == "" {
		return nil, fmt.Errorf("任务未完成或响应中无视频 URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, err
	}
	return httpVideoUpstream.Do(req)
}

func forwardKlingHTTP(ctx context.Context, baseURL, apiKey, method, relativePath string, body []byte, rawQuery string, passthrough http.Header) (*http.Response, error) {
	bearer, err := klingvideo.BearerToken(apiKey)
	if err != nil {
		return nil, err
	}
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream, baseURL, bearer, method, relativePath, body, rawQuery, passthrough)
}

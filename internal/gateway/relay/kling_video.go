package relay

import (
	"bytes"
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

// ForwardKlingVideoTaskHTTP 查询/删除可灵视频任务；storedRest 为创建时记录的相对路径（如 videos/effects）。
func ForwardKlingVideoTaskHTTP(ctx context.Context, baseURL, apiKey, method, taskID string, kind klingvideo.CreateKind, storedRest, rawQuery string, passthrough http.Header) (*http.Response, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("empty task id")
	}
	var path string
	if strings.TrimSpace(storedRest) != "" {
		path = klingvideo.QueryPathWithStoredRest(storedRest, taskID)
	} else {
		path = kind.QueryPath(taskID)
		if kind == "" {
			path = klingvideo.CreateText2Video.QueryPath(taskID)
		}
	}
	resp, err := forwardKlingHTTP(ctx, baseURL, apiKey, method, path, nil, rawQuery, passthrough)
	if err != nil {
		return nil, err
	}
	if storedRest == "" && kind == "" && resp != nil && resp.StatusCode == http.StatusNotFound {
		DrainBody(resp.Body)
		alt := klingvideo.CreateImage2Video.QueryPath(taskID)
		return forwardKlingHTTP(ctx, baseURL, apiKey, method, alt, nil, rawQuery, passthrough)
	}
	return resp, nil
}

// ForwardKlingVideoContent 任务完成后从 video_url 拉取 MP4。
func ForwardKlingVideoContent(ctx context.Context, baseURL, apiKey, taskID string, kind klingvideo.CreateKind, storedRest string, passthrough http.Header) (*http.Response, error) {
	taskResp, err := ForwardKlingVideoTaskHTTP(ctx, baseURL, apiKey, http.MethodGet, taskID, kind, storedRest, "", passthrough)
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

// ForwardKlingNativeHTTP 透传可灵原生 API（相对路径如 v1/videos/text2video）。
func ForwardKlingNativeHTTP(ctx context.Context, baseURL, apiKey, method, relativePath string, body []byte, rawQuery string, passthrough http.Header) (*http.Response, error) {
	return forwardKlingHTTP(ctx, baseURL, apiKey, method, relativePath, body, rawQuery, passthrough)
}

// ForwardKlingNativeMultipart 透传 multipart 请求。
func ForwardKlingNativeMultipart(ctx context.Context, baseURL, apiKey, method, relativePath, contentType string, body []byte, passthrough http.Header) (*http.Response, error) {
	bearer, err := klingvideo.BearerToken(apiKey)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(baseURL, "/")
	rel := strings.Trim(strings.TrimSpace(relativePath), "/")
	url := base + "/" + rel
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = int64(len(body))
	applyPassthroughHeaders(req, passthrough)
	return httpVideoUpstream.Do(req)
}

func forwardKlingHTTP(ctx context.Context, baseURL, apiKey, method, relativePath string, body []byte, rawQuery string, passthrough http.Header) (*http.Response, error) {
	bearer, err := klingvideo.BearerToken(apiKey)
	if err != nil {
		return nil, err
	}
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream, baseURL, bearer, method, relativePath, body, rawQuery, passthrough, true)
}

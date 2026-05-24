package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"tracehub/internal/pkg/volcvideo"
)

const volcVideoTasksPath = "contents/generations/tasks"

// ForwardVolcVideoCreate POST …/contents/generations/tasks（火山方舟 Seedance）。
func ForwardVolcVideoCreate(ctx context.Context, baseURL, apiKey string, body []byte) (*http.Response, error) {
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream, baseURL, apiKey, http.MethodPost, volcVideoTasksPath, body, "", nil, true)
}

// ForwardVolcVideoTaskHTTP GET/DELETE …/contents/generations/tasks/{id}（或列表 GET …/tasks）。
func ForwardVolcVideoTaskHTTP(ctx context.Context, baseURL, apiKey, method, taskID string, rawQuery string, passthrough http.Header) (*http.Response, error) {
	rel := volcVideoTasksPath
	if strings.TrimSpace(taskID) != "" {
		rel = volcVideoTasksPath + "/" + strings.Trim(strings.TrimSpace(taskID), "/")
	}
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream, baseURL, apiKey, method, rel, nil, rawQuery, passthrough, true)
}

// ForwardVolcVideoContent 查询任务完成后从 content.video_url 拉取 MP4。
func ForwardVolcVideoContent(ctx context.Context, baseURL, apiKey, taskID string, passthrough http.Header) (*http.Response, error) {
	taskResp, err := ForwardVolcVideoTaskHTTP(ctx, baseURL, apiKey, http.MethodGet, taskID, "", passthrough)
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
	videoURL, ok := volcvideo.ExtractVideoURLFromTaskBody(taskBody)
	if !ok || videoURL == "" {
		return nil, fmt.Errorf("任务未完成或响应中无 content.video_url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, err
	}
	return httpVideoUpstream.Do(req)
}

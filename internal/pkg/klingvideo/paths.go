package klingvideo

import "strings"

// 可灵视频 API 相对路径（拼在 api_base_url 后）。
const (
	PathText2Video  = "v1/videos/text2video"
	PathImage2Video = "v1/videos/image2video"
)

// CreateKind 创建任务使用的可灵子 API。
type CreateKind string

const (
	CreateText2Video  CreateKind = "text2video"
	CreateImage2Video CreateKind = "image2video"
)

func (k CreateKind) QueryPath(taskID string) string {
	taskID = trimTaskID(taskID)
	switch k {
	case CreateImage2Video:
		return PathImage2Video + "/" + taskID
	default:
		return PathText2Video + "/" + taskID
	}
}

func trimTaskID(id string) string {
	id = strings.Trim(strings.TrimSpace(id), "/")
	for _, p := range []string{PathText2Video + "/", PathImage2Video + "/"} {
		if strings.HasPrefix(id, p) {
			return strings.TrimPrefix(id, p)
		}
	}
	return id
}

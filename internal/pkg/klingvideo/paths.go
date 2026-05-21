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
	CreateExtend      CreateKind = "extend"
	CreateLipSync     CreateKind = "lip_sync"
	CreateEffects     CreateKind = "effects"
)

func (k CreateKind) QueryPath(taskID string) string {
	taskID = trimTaskID(taskID)
	switch k {
	case CreateImage2Video:
		return PathImage2Video + "/" + taskID
	case CreateExtend:
		return "v1/videos/extend/" + taskID
	case CreateLipSync:
		return "v1/videos/lip-sync/" + taskID
	case CreateEffects:
		return "v1/videos/effects/" + taskID
	case CreateText2Video, "":
		return PathText2Video + "/" + taskID
	default:
		if strings.HasPrefix(string(k), "videos/") {
			return "v1/" + string(k) + "/" + taskID
		}
		if s := strings.TrimSpace(string(k)); s != "" {
			return "v1/videos/" + s + "/" + taskID
		}
		return PathText2Video + "/" + taskID
	}
}

// QueryPathWithStoredRest 按创建时的完整相对路径查询（meta.KlingPath 如 videos/effects）。
func QueryPathWithStoredRest(storedRest, taskID string) string {
	storedRest = strings.Trim(strings.TrimSpace(storedRest), "/")
	taskID = trimTaskID(taskID)
	if storedRest == "" {
		return CreateText2Video.QueryPath(taskID)
	}
	base := "v1/" + storedRest
	if strings.HasSuffix(base, "/") {
		return base + taskID
	}
	return base + "/" + taskID
}

func trimTaskID(id string) string {
	id = strings.Trim(strings.TrimSpace(id), "/")
	for _, p := range []string{
		PathText2Video + "/",
		PathImage2Video + "/",
		"v1/videos/text2video/",
		"v1/videos/image2video/",
	} {
		if strings.HasPrefix(id, p) {
			return strings.TrimPrefix(id, p)
		}
	}
	return id
}

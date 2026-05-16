package gateway

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"tracehub/internal/service/billing"
)

const maxVideoMultipartMemory = 64 << 20

type videoCreateParsed struct {
	Model      string
	Seconds    string
	Size       string
	PromptText string
}

func parseVideoCreateBody(body []byte) (videoCreateParsed, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return videoCreateParsed{}, err
	}
	out := videoCreateParsed{}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &out.Model)
		out.Model = strings.TrimSpace(out.Model)
	}
	if raw, ok := m["seconds"]; ok {
		var s any
		_ = json.Unmarshal(raw, &s)
		out.Seconds = formatVideoSecondsField(s)
	}
	if raw, ok := m["size"]; ok {
		_ = json.Unmarshal(raw, &out.Size)
	}
	if raw, ok := m["prompt"]; ok {
		_ = json.Unmarshal(raw, &out.PromptText)
	}
	if raw, ok := m["duration"]; ok {
		var d any
		_ = json.Unmarshal(raw, &d)
		if out.Seconds == "" {
			out.Seconds = formatVideoSecondsField(d)
		}
	}
	if out.PromptText == "" {
		out.PromptText = promptFromArkContentRaw(m["content"])
	}
	return out, nil
}

func videoCreateBodyValid(parsed videoCreateParsed, body []byte) bool {
	if strings.TrimSpace(parsed.Model) == "" {
		return false
	}
	if strings.TrimSpace(parsed.PromptText) != "" {
		return true
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return false
	}
	raw, ok := m["content"]
	if !ok {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		return false
	}
	return true
}

func promptFromArkContentRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return ""
	}
	for _, item := range arr {
		if strings.TrimSpace(stringFieldAny(item, "type")) != "text" {
			continue
		}
		if t := strings.TrimSpace(stringFieldAny(item, "text")); t != "" {
			return t
		}
	}
	return ""
}

func stringFieldAny(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

// readMultipartInputReference 读取 multipart 中 input_reference 文件（试玩本地上传首帧/参考图）。
func readMultipartInputReference(form *multipart.Form) (data []byte, mime string, ok bool, err error) {
	if form == nil || form.File == nil {
		return nil, "", false, nil
	}
	files := form.File["input_reference"]
	if len(files) == 0 {
		return nil, "", false, nil
	}
	fh := files[0]
	f, err := fh.Open()
	if err != nil {
		return nil, "", false, err
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, maxVideoMultipartMemory))
	if err != nil {
		return nil, "", false, err
	}
	mime = strings.TrimSpace(fh.Header.Get("Content-Type"))
	return data, mime, true, nil
}

func parseVideoCreateMultipart(form *multipart.Form) videoCreateParsed {
	out := videoCreateParsed{}
	if vals := form.Value["model"]; len(vals) > 0 {
		out.Model = strings.TrimSpace(vals[0])
	}
	if vals := form.Value["seconds"]; len(vals) > 0 {
		out.Seconds = strings.TrimSpace(vals[0])
	}
	if vals := form.Value["size"]; len(vals) > 0 {
		out.Size = strings.TrimSpace(vals[0])
	}
	if vals := form.Value["prompt"]; len(vals) > 0 {
		out.PromptText = strings.TrimSpace(vals[0])
	}
	return out
}

func formatVideoSecondsField(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.Itoa(int(t))
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func videoJobParamsFromParsed(p videoCreateParsed) billing.VideoJobParams {
	return billing.VideoJobParams{
		Seconds: billing.ParseVideoSeconds(p.Seconds),
		Size:    p.Size,
	}
}

func parseVideoIDFromJSON(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	for _, k := range []string{"id", "video_id", "task_id"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// parseVideoBillableSeconds 精调计费：优先上游 usage.seconds，其次对象 seconds/duration。
func parseVideoBillableSeconds(body []byte) int {
	var wrap struct {
		Usage struct {
			Seconds float64 `json:"seconds"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &wrap); err == nil && wrap.Usage.Seconds > 0 {
		sec := int(wrap.Usage.Seconds + 0.5)
		if sec < 1 {
			return 1
		}
		return sec
	}
	_, sec := parseVideoStatusFromJSON(body)
	return sec
}

func parseVideoStatusFromJSON(body []byte) (status string, seconds int) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", 0
	}
	if v, ok := m["status"].(string); ok {
		status = strings.ToLower(strings.TrimSpace(v))
		if status == "succeeded" || status == "success" {
			status = "completed"
		}
	}
	if v, ok := m["seconds"]; ok {
		switch t := v.(type) {
		case string:
			seconds = billing.ParseVideoSeconds(t)
		case float64:
			seconds = int(t)
		}
	}
	if seconds <= 0 {
		if v, ok := m["duration"]; ok {
			switch t := v.(type) {
			case string:
				seconds = billing.ParseVideoSeconds(t)
			case float64:
				seconds = int(t)
			}
		}
	}
	return status, seconds
}

func parseVideoSizeFromJSON(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	if v, ok := m["size"].(string); ok {
		return v
	}
	return ""
}

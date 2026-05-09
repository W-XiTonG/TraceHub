package modelcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/ent/modelcatalogsource"
	"tracehub/internal/service/channelsecret"

	"github.com/google/uuid"
)

type openAIModelsResp struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// SyncSource 从配置的 HTTP 源拉取模型并写入 model_catalog_entries（按 model_id upsert）。
func SyncSource(ctx context.Context, client *ent.Client, aesKey []byte, sourceID int) (inserted int, updated int, err error) {
	src, err := client.ModelCatalogSource.Get(ctx, sourceID)
	if err != nil {
		return 0, 0, err
	}
	if !src.Enabled {
		return 0, 0, fmt.Errorf("源已禁用")
	}
	if src.ParseMode != modelcatalogsource.ParseModeOpenaiJSON {
		return 0, 0, fmt.Errorf("当前仅支持 parse_mode=openai_json")
	}

	base := strings.TrimSpace(src.BaseURL)
	path := strings.TrimSpace(src.ModelsPath)
	if path == "" {
		path = "/v1/models"
	}
	listURL := joinURL(base, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return 0, 0, err
	}

	switch src.CredentialMode {
	case modelcatalogsource.CredentialModeNone:
		// no auth
	case modelcatalogsource.CredentialModeStatic:
		key := channelsecret.Plaintext(aesKey, src.APIKeyEncrypted)
		if key == "" {
			return 0, 0, fmt.Errorf("未配置静态 API Key")
		}
		req.Header.Set("Authorization", "Bearer "+key)
	case modelcatalogsource.CredentialModeChannel:
		if src.ChannelID == 0 {
			return 0, 0, fmt.Errorf("未绑定渠道")
		}
		ch, err := client.Channel.Get(ctx, src.ChannelID)
		if err != nil {
			return 0, 0, fmt.Errorf("渠道不存在: %w", err)
		}
		key := channelsecret.Plaintext(aesKey, ch.APIKey)
		if key == "" {
			return 0, 0, fmt.Errorf("渠道未配置可用的 API Key")
		}
		req.Header.Set("Authorization", "Bearer "+key)
		chBase := strings.TrimSpace(ch.APIBaseURL)
		if chBase != "" {
			listURL = joinURL(chBase, path)
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
			if err != nil {
				return 0, 0, err
			}
			req.Header.Set("Authorization", "Bearer "+key)
		}
	default:
		return 0, 0, fmt.Errorf("未知凭证模式")
	}

	for k, v := range src.ExtraHeaders {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())

	httpClient := &http.Client{Timeout: 45 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		_ = client.ModelCatalogSource.UpdateOneID(src.ID).
			SetLastSyncAt(time.Now()).
			SetLastSyncOk(false).
			SetLastSyncMessage(err.Error()).
			Exec(ctx)
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
		_ = client.ModelCatalogSource.UpdateOneID(src.ID).
			SetLastSyncAt(time.Now()).
			SetLastSyncOk(false).
			SetLastSyncMessage(msg).
			Exec(ctx)
		return 0, 0, fmt.Errorf("%s", msg)
	}

	var parsed openAIModelsResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		_ = client.ModelCatalogSource.UpdateOneID(src.ID).
			SetLastSyncAt(time.Now()).
			SetLastSyncOk(false).
			SetLastSyncMessage("解析 JSON 失败: "+err.Error()).
			Exec(ctx)
		return 0, 0, fmt.Errorf("解析响应: %w", err)
	}
	if parsed.Object != "list" && len(parsed.Data) == 0 {
		_ = client.ModelCatalogSource.UpdateOneID(src.ID).
			SetLastSyncAt(time.Now()).
			SetLastSyncOk(false).
			SetLastSyncMessage("响应格式非 OpenAI models 列表").
			Exec(ctx)
		return 0, 0, fmt.Errorf("非 OpenAI models 列表格式")
	}

	now := time.Now()
	prov := strings.TrimSpace(src.ProviderSlug)
	for _, m := range parsed.Data {
		if m.ID == "" {
			continue
		}
		meta := map[string]any{
			"object":   m.Object,
			"created":  m.Created,
			"owned_by": m.OwnedBy,
		}
		existing, err := client.ModelCatalogEntry.Query().
			Where(modelcatalogentry.ModelIDEQ(m.ID)).
			Only(ctx)
		if ent.IsNotFound(err) {
			cr := client.ModelCatalogEntry.Create().
				SetModelID(m.ID).
				SetDisplayName(m.ID).
				SetProviderSlug(prov).
				SetSourceID(src.ID).
				SetMetadata(meta).
				SetVisible(true).
				SetEnabled(true).
				SetSyncedAt(now)
			if ob := strings.TrimSpace(m.OwnedBy); ob != "" {
				cr.SetOwnedBy(ob)
			}
			_, err = cr.Save(ctx)
			if err == nil {
				inserted++
			}
		} else if err == nil {
			up := client.ModelCatalogEntry.UpdateOne(existing).
				SetDisplayName(coalesceStr(existing.DisplayName, m.ID)).
				SetProviderSlug(prov).
				SetSourceID(src.ID).
				SetMetadata(meta).
				SetSyncedAt(now)
			if ob := strings.TrimSpace(m.OwnedBy); ob != "" {
				up.SetOwnedBy(ob)
			} else {
				up.ClearOwnedBy()
			}
			if err := up.Exec(ctx); err == nil {
				updated++
			}
		}
	}

	msg := fmt.Sprintf("同步完成: 新增 %d, 更新 %d", inserted, updated)
	_ = client.ModelCatalogSource.UpdateOneID(src.ID).
		SetLastSyncAt(now).
		SetLastSyncOk(true).
		SetLastSyncMessage(msg).
		Exec(ctx)
	return inserted, updated, nil
}

func coalesceStr(cur, fallback string) string {
	if strings.TrimSpace(cur) != "" {
		return cur
	}
	return fallback
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

package adminhandler

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// BrandIconPublicURLPrefix 本地上传图标对外 URL 前缀（与 gin Static 挂载一致）。
	BrandIconPublicURLPrefix = "/uploads/site-brand"
	brandIconMaxBytes        = 2 << 20 // 2 MiB
)

// BrandUploadDir 品牌图标本地上传目录（绝对路径）；配置空时默认 data/site_brand。
func BrandUploadDir(cfg *config.Config) string {
	d := strings.TrimSpace(cfg.Site.BrandUploadDir)
	if d == "" {
		d = "data/site_brand"
	}
	abs, err := filepath.Abs(d)
	if err != nil {
		return d
	}
	return abs
}

// detectBrandUploadExt 仅允许常见栅格/WebP，避免 SVG 脚本通过 <img> 同源加载风险。
func detectBrandUploadExt(data []byte) (ext string, ok bool) {
	if len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return ".png", true
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return ".jpg", true
	}
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return ".gif", true
	}
	if len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) {
		if len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")) {
			return ".webp", true
		}
	}
	return "", false
}

func brandIconPublicURL(filename string) string {
	return BrandIconPublicURLPrefix + "/" + filename
}

func isSiteUploadLibraryFilename(name string) bool {
	if name == "" || strings.Contains(name, "..") {
		return false
	}
	if filepath.Base(name) != name {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
	default:
		return false
	}
	return strings.HasPrefix(name, "site-brand-") ||
		strings.HasPrefix(name, "site-contact-qr-") ||
		strings.HasPrefix(name, "site-favicon-") ||
		strings.HasPrefix(name, "site-og-") ||
		strings.HasPrefix(name, "site-contact-card-")
}

// configKeyForUploadFilename 按文件名前缀返回关联的 system_configs 键；联系 QR 仅填表不落独立键。
func configKeyForUploadFilename(filename string) string {
	switch {
	case strings.HasPrefix(filename, "site-brand-"):
		return sitecfg.SystemConfigKeyBrandIconURL
	case strings.HasPrefix(filename, "site-favicon-"):
		return sitecfg.SystemConfigKeyBrandFaviconURL
	case strings.HasPrefix(filename, "site-og-"):
		return sitecfg.SystemConfigKeyBrandOGImageURL
	case strings.HasPrefix(filename, "site-contact-card-"):
		return sitecfg.SystemConfigKeyBrandContactCardURL
	default:
		return ""
	}
}

func storedURLForConfigKey(ctx context.Context, client *ent.Client, configKey string) string {
	switch configKey {
	case sitecfg.SystemConfigKeyBrandIconURL:
		return sitecfg.StoredBrandIconURL(ctx, client)
	case sitecfg.SystemConfigKeyBrandFaviconURL:
		return sitecfg.StoredBrandFaviconURL(ctx, client)
	case sitecfg.SystemConfigKeyBrandOGImageURL:
		return sitecfg.StoredBrandOGImageURL(ctx, client)
	case sitecfg.SystemConfigKeyBrandContactCardURL:
		return sitecfg.StoredBrandContactCardURL(ctx, client)
	default:
		return ""
	}
}

// isBrandLibraryFilename 兼容旧名：本地上传图库中的栅格图文件名。
func isBrandLibraryFilename(name string) bool {
	return isSiteUploadLibraryFilename(name)
}

// GetBrandIconLibrary GET /api/admin/site-brand/icons — 列出本地上传目录内保留的品牌图标（按修改时间新到旧）。
func GetBrandIconLibrary(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		root := BrandUploadDir(cfg)
		de, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				httpx.OK(c, gin.H{"icons": []gin.H{}})
				return
			}
			httpx.ServerError(c, "读取图标目录失败")
			return
		}
		type iconRow struct {
			URL        string `json:"url"`
			UploadedAt int64  `json:"uploaded_at"`
			Filename   string `json:"filename"`
		}
		var rows []iconRow
		for _, e := range de {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !isSiteUploadLibraryFilename(name) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			url := brandIconPublicURL(name)
			if err := sitecfg.ValidateBrandIconURL(url); err != nil {
				continue
			}
			rows = append(rows, iconRow{
				URL:        url,
				UploadedAt: info.ModTime().Unix(),
				Filename:   name,
			})
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].UploadedAt != rows[j].UploadedAt {
				return rows[i].UploadedAt > rows[j].UploadedAt
			}
			return rows[i].URL > rows[j].URL
		})
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, gin.H{
				"url":         r.URL,
				"uploaded_at": r.UploadedAt,
				"filename":    r.Filename,
			})
		}
		httpx.OK(c, gin.H{"icons": out})
	}
}

// DeleteBrandIcon DELETE /api/admin/site-brand/icons/:filename — 删除本地上传文件；若 URL 与对应配置一致则清空该键。
func DeleteBrandIcon(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		filename := strings.TrimSpace(c.Param("filename"))
		if filename == "" || !isSiteUploadLibraryFilename(filename) {
			httpx.BadRequest(c, "无效的文件名")
			return
		}
		root := BrandUploadDir(cfg)
		dest := filepath.Join(root, filename)
		rel, err := filepath.Rel(root, dest)
		if err != nil || strings.HasPrefix(rel, "..") {
			httpx.BadRequest(c, "无效的路径")
			return
		}
		if _, err := os.Stat(dest); err != nil {
			if os.IsNotExist(err) {
				httpx.BadRequest(c, "文件不存在")
				return
			}
			httpx.ServerError(c, "读取文件失败")
			return
		}

		ctx := c.Request.Context()
		deletedURL := brandIconPublicURL(filename)
		cleared := false
		if configKey := configKeyForUploadFilename(filename); configKey != "" {
			stored := storedURLForConfigKey(ctx, client, configKey)
			if strings.TrimSpace(stored) == deletedURL {
				ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(configKey)).Only(ctx)
				if err != nil {
					if !ent.IsNotFound(err) {
						httpx.ServerError(c, "读取配置失败")
						return
					}
				} else {
					if err := client.SystemConfig.UpdateOne(ex).SetConfigValue("").Exec(ctx); err != nil {
						httpx.ServerError(c, "更新配置失败")
						return
					}
					cleared = true
				}
			}
		}

		if err := os.Remove(dest); err != nil {
			httpx.ServerError(c, "删除文件失败："+err.Error())
			return
		}

		auditLog(client, adm.ID, "site.brand_icon_delete", "site_brand", filename, map[string]any{"url": deletedURL, "cleared_config": cleared}, c)
		httpx.OK(c, gin.H{"cleared_config": cleared})
	}
}

type siteRasterUploadOpts struct {
	filenamePrefix string
	configKey      string
	auditAction    string
}

func upsertBrandConfigURL(ctx context.Context, client *ent.Client, configKey, url, description string) error {
	ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(configKey)).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = client.SystemConfig.Create().
			SetConfigKey(configKey).
			SetConfigValue(url).
			SetDescription(description).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	return client.SystemConfig.UpdateOne(ex).SetConfigValue(url).Exec(ctx)
}

func postSiteRasterUpload(client *ent.Client, cfg *config.Config, opts siteRasterUploadOpts) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		if err := c.Request.ParseMultipartForm(brandIconMaxBytes + 1); err != nil {
			httpx.BadRequest(c, "无法解析上传表单")
			return
		}
		fh, err := c.FormFile("file")
		if err != nil || fh == nil {
			httpx.BadRequest(c, "缺少文件字段 file")
			return
		}
		if fh.Size > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		src, err := fh.Open()
		if err != nil {
			httpx.BadRequest(c, "无法读取上传文件")
			return
		}
		defer func() { _ = src.Close() }()

		data, err := io.ReadAll(io.LimitReader(src, brandIconMaxBytes+1))
		if err != nil {
			httpx.ServerError(c, "读取文件失败")
			return
		}
		if len(data) > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		ext, ok := detectBrandUploadExt(data)
		if !ok {
			httpx.BadRequest(c, "仅支持 PNG、JPEG、GIF、WebP 图片")
			return
		}

		root := BrandUploadDir(cfg)
		if err := os.MkdirAll(root, 0o755); err != nil {
			httpx.ServerError(c, "无法创建上传目录："+err.Error())
			return
		}

		filename := opts.filenamePrefix + uuid.New().String() + ext
		dest := filepath.Join(root, filename)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			httpx.ServerError(c, "保存文件失败："+err.Error())
			return
		}

		url := brandIconPublicURL(filename)
		if err := sitecfg.ValidateBrandIconURL(url); err != nil {
			httpx.ServerError(c, "内部错误：生成 URL 无效")
			return
		}

		ctx := c.Request.Context()
		if err := upsertBrandConfigURL(ctx, client, opts.configKey, url, opts.configKey+" (uploaded)"); err != nil {
			httpx.ServerError(c, "更新配置失败")
			return
		}

		auditLog(client, adm.ID, opts.auditAction, "system_config", opts.configKey, map[string]any{"url": url}, c)
		httpx.OK(c, gin.H{"url": url})
	}
}

// PostBrandIconUpload POST /api/admin/site-upload/brand-icon
func PostBrandIconUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postSiteRasterUpload(client, cfg, siteRasterUploadOpts{
		filenamePrefix: "site-brand-",
		configKey:      sitecfg.SystemConfigKeyBrandIconURL,
		auditAction:    "site.brand_icon_upload",
	})
}

// PostBrandFaviconUpload POST /api/admin/site-upload/brand-favicon
func PostBrandFaviconUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postSiteRasterUpload(client, cfg, siteRasterUploadOpts{
		filenamePrefix: "site-favicon-",
		configKey:      sitecfg.SystemConfigKeyBrandFaviconURL,
		auditAction:    "site.brand_favicon_upload",
	})
}

// PostBrandOGImageUpload POST /api/admin/site-upload/brand-og-image
func PostBrandOGImageUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postSiteRasterUpload(client, cfg, siteRasterUploadOpts{
		filenamePrefix: "site-og-",
		configKey:      sitecfg.SystemConfigKeyBrandOGImageURL,
		auditAction:    "site.brand_og_upload",
	})
}

// PostContactCardUpload POST /api/admin/site-upload/contact-card
func PostContactCardUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return postSiteRasterUpload(client, cfg, siteRasterUploadOpts{
		filenamePrefix: "site-contact-card-",
		configKey:      sitecfg.SystemConfigKeyBrandContactCardURL,
		auditAction:    "site.contact_card_upload",
	})
}

// PostContactQRUpload POST /api/admin/site-upload/contact-qr — 上传联系我们二维码图片；仅返回 URL，不修改站点图标配置。
func PostContactQRUpload(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		if err := c.Request.ParseMultipartForm(brandIconMaxBytes + 1); err != nil {
			httpx.BadRequest(c, "无法解析上传表单")
			return
		}
		fh, err := c.FormFile("file")
		if err != nil || fh == nil {
			httpx.BadRequest(c, "缺少文件字段 file")
			return
		}
		if fh.Size > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		src, err := fh.Open()
		if err != nil {
			httpx.BadRequest(c, "无法读取上传文件")
			return
		}
		defer func() { _ = src.Close() }()

		data, err := io.ReadAll(io.LimitReader(src, brandIconMaxBytes+1))
		if err != nil {
			httpx.ServerError(c, "读取文件失败")
			return
		}
		if len(data) > brandIconMaxBytes {
			httpx.BadRequest(c, "文件过大（最大 2MB）")
			return
		}
		ext, ok := detectBrandUploadExt(data)
		if !ok {
			httpx.BadRequest(c, "仅支持 PNG、JPEG、GIF、WebP 图片")
			return
		}

		root := BrandUploadDir(cfg)
		if err := os.MkdirAll(root, 0o755); err != nil {
			httpx.ServerError(c, "无法创建上传目录："+err.Error())
			return
		}

		filename := "site-contact-qr-" + uuid.New().String() + ext
		dest := filepath.Join(root, filename)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			httpx.ServerError(c, "保存文件失败："+err.Error())
			return
		}

		url := brandIconPublicURL(filename)
		if err := sitecfg.ValidateBrandIconURL(url); err != nil {
			httpx.ServerError(c, "内部错误：生成 URL 无效")
			return
		}

		auditLog(client, adm.ID, "site.contact_qr_upload", "site_brand", filename, map[string]any{"url": url}, c)
		httpx.OK(c, gin.H{"url": url})
	}
}

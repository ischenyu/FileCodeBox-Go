// Package storage WebDAV 存储实现
// 与 Python 版 core/storage.py 的 WebDAVFileStorage 保持一致
package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/ischenyu/FileCodeBox-Go/internal/models"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// WebDAVStorage WebDAV 存储后端
type WebDAVStorage struct {
	baseURL  string
	username string
	password string
	rootPath string
	client   *http.Client
}

// WebDAVConfig WebDAV 存储配置
type WebDAVConfig struct {
	URL      string
	Username string
	Password string
	RootPath string
}

// NewWebDAVStorage 创建 WebDAV 存储实例
func NewWebDAVStorage(cfg WebDAVConfig) *WebDAVStorage {
	return &WebDAVStorage{
		baseURL:  strings.TrimRight(cfg.URL, "/"),
		username: cfg.Username,
		password: cfg.Password,
		rootPath: cfg.RootPath,
		client:   &http.Client{},
	}
}

// buildURL 构建完整的 WebDAV URL
func (w *WebDAVStorage) buildURL(p string) string {
	cleanPath := strings.TrimLeft(strings.ReplaceAll(p, "\\", "/"), "/")
	return w.baseURL + "/" + cleanPath
}

// doRequest 执行带 Basic Auth 的 WebDAV 请求
func (w *WebDAVStorage) doRequest(method, urlStr string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(w.username, w.password)
	return w.client.Do(req)
}

// mkdirP 递归创建 WebDAV 目录
func (w *WebDAVStorage) mkdirP(dirPath string) error {
	parts := strings.Split(strings.Trim(dirPath, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = current + "/" + part
		}
		dirURL := w.buildURL(current)

		// 检查目录是否存在
		resp, err := w.doRequest("HEAD", dirURL, nil)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode == 404 {
			// 创建目录
			resp, err = w.doRequest("MKCOL", dirURL, nil)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode >= 400 && resp.StatusCode != 409 {
				return fmt.Errorf("创建目录失败: %d", resp.StatusCode)
			}
		}
	}
	return nil
}

// SaveFile 上传文件到 WebDAV
func (w *WebDAVStorage) SaveFile(file *multipart.FileHeader, savePath string) error {
	dir := path.Dir(savePath)
	filename := utils.SanitizeFilename(path.Base(savePath))
	safePath := path.Join(dir, filename)

	if err := w.mkdirP(dir); err != nil {
		return fmt.Errorf("创建WebDAV目录失败: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	fileURL := w.buildURL(safePath)
	resp, err := w.doRequest("PUT", fileURL, src)
	if err != nil {
		return fmt.Errorf("WebDAV上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WebDAV上传失败(%d): %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}
	return nil
}

// DeleteFile 删除 WebDAV 文件
func (w *WebDAVStorage) DeleteFile(fileCode *models.FileCodes) error {
	fileURL := w.buildURL(fileCode.GetFilePath())
	resp, err := w.doRequest("DELETE", fileURL, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	// 404 也视为成功
	return nil
}

// GetFileURL 获取中转下载URL
func (w *WebDAVStorage) GetFileURL(fileCode *models.FileCodes) string {
	return utils.GetFileURL(fileCode.Code, "")
}

// GetFileResponse WebDAV 文件不在本地，返回空路径
func (w *WebDAVStorage) GetFileResponse(fileCode *models.FileCodes) (string, error) {
	return "", fmt.Errorf("WebDAV文件需通过代理下载: %s", fileCode.GetFilePath())
}

// SaveChunk 保存分片到 WebDAV
func (w *WebDAVStorage) SaveChunk(uploadID string, chunkIndex int, chunkData []byte, chunkHash string, savePath string) error {
	chunkDir := path.Join(path.Dir(savePath), "chunks", uploadID)
	chunkPath := fmt.Sprintf("%s/%d.part", chunkDir, chunkIndex)

	if err := w.mkdirP(chunkDir); err != nil {
		return err
	}

	chunkURL := w.buildURL(chunkPath)
	resp, err := w.doRequest("PUT", chunkURL, bytes.NewReader(chunkData))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("上传分片失败: %d", resp.StatusCode)
	}
	return nil
}

// MergeChunks 合并 WebDAV 分片
func (w *WebDAVStorage) MergeChunks(uploadID string, chunkInfo *models.UploadChunk, savePath string) (string, string, error) {
	chunkDir := path.Join(path.Dir(savePath), "chunks", uploadID)

	// 逐个下载分片并合并到内存
	var buf bytes.Buffer
	for i := 0; i < chunkInfo.TotalChunks; i++ {
		chunkPath := fmt.Sprintf("%s/%d.part", chunkDir, i)
		chunkURL := w.buildURL(chunkPath)

		resp, err := w.doRequest("GET", chunkURL, nil)
		if err != nil {
			return "", "", fmt.Errorf("下载分片%d失败: %w", i, err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", "", fmt.Errorf("下载分片%d失败: %d", i, resp.StatusCode)
		}
		buf.Write(data)
	}

	// 上传合并后的文件
	fileURL := w.buildURL(savePath)
	resp, err := w.doRequest("PUT", fileURL, &buf)
	if err != nil {
		return "", "", fmt.Errorf("上传合并文件失败: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("上传合并文件失败: %d", resp.StatusCode)
	}

	return savePath, "", nil
}

// GeneratePresignedUploadURL WebDAV 不支持直传
func (w *WebDAVStorage) GeneratePresignedUploadURL(savePath string, expiresIn int) (string, error) {
	return "", nil
}

// FileExists 检查 WebDAV 文件是否存在
func (w *WebDAVStorage) FileExists(savePath string) (bool, error) {
	fileURL := w.buildURL(savePath)
	resp, err := w.doRequest("HEAD", fileURL, nil)
	if err != nil {
		return false, nil
	}
	resp.Body.Close()
	return resp.StatusCode == 200, nil
}

// CleanChunks 清理 WebDAV 临时分片
func (w *WebDAVStorage) CleanChunks(uploadID string, savePath string) error {
	// WebDAV 简化实现：通过 PROPFIND 列出并 DELETE
	chunkDir := path.Join(path.Dir(savePath), "chunks", uploadID)
	chunkURL := w.buildURL(chunkDir)
	// 尝试直接删除整个分片目录
	resp, err := w.doRequest("DELETE", chunkURL, nil)
	if err != nil {
		return nil
	}
	resp.Body.Close()
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensure imports
var _ = url.Parse

// Package storage 提供存储后端注册与工厂方法
// 与 Python 版 core/storage.py 末尾的 storages 字典保持一致
package storage

import (
	"fmt"
	"log/slog"

	"github.com/ischenyu/internal/config"
)

// StorageFactory 存储工厂函数类型
type StorageFactory func(cfg *config.Settings) (FileStorageInterface, error)

// 支持的存储后端类型
const (
	StorageLocal    = "local"
	StorageS3       = "s3"
	StorageOneDrive = "onedrive"
	StorageOpenDAL  = "opendal"
	StorageWebDAV   = "webdav"
)

// registry 存储后端注册表
var registry = map[string]StorageFactory{
	StorageLocal:    newLocal,
	StorageS3:       newS3,
	StorageOneDrive: newOneDrive,
	StorageOpenDAL:  newOpenDAL,
	StorageWebDAV:   newWebDAV,
}

// NewStorage 根据配置创建对应的存储后端实例
func NewStorage(cfg *config.Settings) (FileStorageInterface, error) {
	storageType := cfg.GetString("file_storage")
	if storageType == "" {
		storageType = StorageLocal
	}

	factory, ok := registry[storageType]
	if !ok {
		return nil, fmt.Errorf("不支持的存储类型: %s", storageType)
	}

	storage, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建存储后端失败[%s]: %w", storageType, err)
	}

	slog.Info("存储后端已初始化", "type", storageType)
	return storage, nil
}

// newLocal 创建本地存储
func newLocal(cfg *config.Settings) (FileStorageInterface, error) {
	dataRoot := cfg.GetDataRoot()
	return NewLocalStorage(dataRoot), nil
}

// newS3 创建 S3 存储
func newS3(cfg *config.Settings) (FileStorageInterface, error) {
	s3cfg := S3Config{
		AccessKeyID:     cfg.GetString("s3_access_key_id"),
		SecretAccessKey: cfg.GetString("s3_secret_access_key"),
		BucketName:      cfg.GetString("s3_bucket_name"),
		EndpointURL:     cfg.GetString("s3_endpoint_url"),
		RegionName:      cfg.GetString("s3_region_name"),
		SessionToken:    cfg.GetString("aws_session_token"),
		SignatureVer:    cfg.GetString("s3_signature_version"),
		AddressingStyle: cfg.GetString("s3_addressing_style"),
	}

	if s3cfg.EndpointURL == "" && cfg.GetString("s3_hostname") != "" {
		s3cfg.EndpointURL = "https://" + cfg.GetString("s3_hostname")
	}

	return NewS3Storage(s3cfg)
}

// newOneDrive 创建 OneDrive 存储
func newOneDrive(cfg *config.Settings) (FileStorageInterface, error) {
	odCfg := OneDriveConfig{
		Domain:   cfg.GetString("onedrive_domain"),
		ClientID: cfg.GetString("onedrive_client_id"),
		Username: cfg.GetString("onedrive_username"),
		Password: cfg.GetString("onedrive_password"),
		RootPath: cfg.GetString("onedrive_root_path"),
		Proxy:    cfg.GetBool("onedrive_proxy"),
	}
	return NewOneDriveStorage(odCfg)
}

// newOpenDAL 创建 OpenDAL 存储
func newOpenDAL(cfg *config.Settings) (FileStorageInterface, error) {
	return NewOpenDALStorage("", ""), nil
}

// newWebDAV 创建 WebDAV 存储
func newWebDAV(cfg *config.Settings) (FileStorageInterface, error) {
	webdavCfg := WebDAVConfig{
		URL:      cfg.GetString("webdav_url"),
		Username: cfg.GetString("webdav_username"),
		Password: cfg.GetString("webdav_password"),
		RootPath: cfg.GetString("webdav_root_path"),
	}
	return NewWebDAVStorage(webdavCfg), nil
}

// Package models 定义数据库模型
// 与 Python 版 apps/base/models.py 保持一致
package models

import (
	"database/sql"
	"time"
)

// FileCodes 文件/文本分享记录
// 对应 Python 版 FileCodes 模型
type FileCodes struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string         `gorm:"column:code;uniqueIndex;size:255;not null" json:"code"`            // 提取码
	Prefix       string         `gorm:"column:prefix;size:255;default:''" json:"prefix"`                   // 文件名前缀
	Suffix       string         `gorm:"column:suffix;size:255;default:''" json:"suffix"`                   // 文件名后缀（扩展名）
	UUIDFileName *string        `gorm:"column:uuid_file_name;size:255" json:"uuid_file_name"`             // 存储用文件名
	FilePath     *string        `gorm:"column:file_path;size:255" json:"file_path"`                       // 文件存储路径
	Size         int64          `gorm:"column:size;default:0" json:"size"`                                // 文件大小（字节）
	Text         *string        `gorm:"column:text;type:text" json:"text"`                                // 文本内容（文本分享时使用）
	ExpiredAt    *time.Time     `gorm:"column:expired_at" json:"expired_at"`                              // 过期时间
	ExpiredCount int            `gorm:"column:expired_count;default:0" json:"expired_count"`              // 剩余可下载次数
	UsedCount    int            `gorm:"column:used_count;default:0" json:"used_count"`                    // 已使用次数
	CreatedAt    time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`               // 创建时间
	FileHash     *string        `gorm:"column:file_hash;size:64" json:"file_hash"`                        // 文件 SHA256 哈希
	IsChunked    bool           `gorm:"column:is_chunked;default:false" json:"is_chunked"`                // 是否切片上传
	UploadID     *string        `gorm:"column:upload_id;size:36" json:"upload_id"`                        // 切片上传会话ID
}

// TableName 指定表名
func (FileCodes) TableName() string {
	return "file_codes"
}

// IsExpired 判断文件/文本是否已过期
func (f *FileCodes) IsExpired() bool {
	// 无过期时间 → 永不过期
	if f.ExpiredAt == nil {
		return false
	}
	// 有过期时间且 expired_count < 0 → 按时间判断
	if f.ExpiredAt != nil && f.ExpiredCount < 0 {
		return f.ExpiredAt.Before(time.Now())
	}
	// 否则按次数判断
	return f.ExpiredCount <= 0
}

// GetFilePath 返回完整的文件存储路径
func (f *FileCodes) GetFilePath() string {
	if f.FilePath == nil || f.UUIDFileName == nil {
		return ""
	}
	return *f.FilePath + "/" + *f.UUIDFileName
}

// UploadChunk 切片上传分片记录
// 对应 Python 版 UploadChunk 模型
// 注意: chunk_index=-1 的记录存的是上传会话元信息
type UploadChunk struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UploadID    string    `gorm:"column:upload_id;index;size:36;not null" json:"upload_id"`   // 上传会话ID
	ChunkIndex  int       `gorm:"column:chunk_index;not null" json:"chunk_index"`             // 分片索引 (-1=元信息)
	ChunkHash   string    `gorm:"column:chunk_hash;size:64;not null" json:"chunk_hash"`       // 分片 SHA256
	TotalChunks int       `gorm:"column:total_chunks;not null" json:"total_chunks"`           // 总分片数
	FileSize    int64     `gorm:"column:file_size;not null" json:"file_size"`                 // 文件总大小
	ChunkSize   int       `gorm:"column:chunk_size;not null" json:"chunk_size"`               // 单个分片大小
	FileName    string    `gorm:"column:file_name;size:255;not null" json:"file_name"`        // 原始文件名
	SavePath    *string   `gorm:"column:save_path;size:512" json:"save_path"`                 // 保存路径
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`         // 创建时间
	Completed   bool      `gorm:"column:completed;default:false" json:"completed"`            // 分片是否上传完成
}

// TableName 指定表名
func (UploadChunk) TableName() string {
	return "upload_chunk"
}

// KeyValue 键值对配置存储
// 对应 Python 版 KeyValue 模型
type KeyValue struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Key       string         `gorm:"column:key;uniqueIndex;size:255;not null" json:"key"` // 键
	Value     sql.NullString `gorm:"column:value;type:json" json:"value"`                  // 值（JSON字符串）
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`   // 创建时间
}

// TableName 指定表名
func (KeyValue) TableName() string {
	return "key_value"
}

// PresignUploadSession 预签名上传会话
// 对应 Python 版 PresignUploadSession 模型
type PresignUploadSession struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UploadID    string    `gorm:"column:upload_id;uniqueIndex;size:36;not null" json:"upload_id"` // 上传会话ID
	FileName    string    `gorm:"column:file_name;size:255;not null" json:"file_name"`            // 原始文件名
	FileSize    int64     `gorm:"column:file_size;not null" json:"file_size"`                     // 文件大小
	SavePath    string    `gorm:"column:save_path;size:512;not null" json:"save_path"`            // 保存路径
	Mode        string    `gorm:"column:mode;size:10;not null" json:"mode"`                       // 模式: "direct" / "proxy"
	ExpireValue int       `gorm:"column:expire_value;default:1" json:"expire_value"`              // 过期数值
	ExpireStyle string    `gorm:"column:expire_style;size:20;default:day" json:"expire_style"`    // 过期类型
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`             // 创建时间
	ExpiresAt   time.Time `gorm:"column:expires_at;not null" json:"expires_at"`                   // 会话过期时间
}

// TableName 指定表名
func (PresignUploadSession) TableName() string {
	return "presign_upload_session"
}

// IsExpired 检查会话是否已过期
func (p *PresignUploadSession) IsExpired() bool {
	return p.ExpiresAt.Before(time.Now())
}

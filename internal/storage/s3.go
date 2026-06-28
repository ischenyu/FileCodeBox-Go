// Package storage S3 兼容对象存储实现
// 与 Python 版 core/storage.py 的 S3FileStorage 保持一致
package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/ischenyu/FileCodeBox-Go/internal/models"
	"github.com/ischenyu/FileCodeBox-Go/internal/utils"
)

// S3Storage AWS S3 兼容对象存储
type S3Storage struct {
	client          *s3.Client
	presignClient   *s3.PresignClient
	bucketName      string
	endpointURL     string
	regionName      string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	signatureVer    string
	addressingStyle string
}

// S3Config S3 存储配置
type S3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	EndpointURL     string
	RegionName      string
	SessionToken    string
	SignatureVer    string
	AddressingStyle string
}

// NewS3Storage 创建 S3 存储实例
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	if cfg.RegionName == "" {
		cfg.RegionName = "auto"
	}
	if cfg.SignatureVer == "" {
		cfg.SignatureVer = "s3v2"
	}
	if cfg.EndpointURL == "" {
		return nil, fmt.Errorf("S3 endpoint URL 不能为空")
	}

	usePathStyle := cfg.AddressingStyle == "path"

	// 使用静态凭证
	credProvider := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.RegionName),
		config.WithCredentialsProvider(credProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.EndpointURL)
		o.UsePathStyle = usePathStyle
	})

	return &S3Storage{
		client:          client,
		presignClient:   s3.NewPresignClient(client),
		bucketName:      cfg.BucketName,
		endpointURL:     cfg.EndpointURL,
		regionName:      cfg.RegionName,
		accessKeyID:     cfg.AccessKeyID,
		secretAccessKey: cfg.SecretAccessKey,
		sessionToken:    cfg.SessionToken,
		signatureVer:    cfg.SignatureVer,
		addressingStyle: cfg.AddressingStyle,
	}, nil
}

// SaveFile 上传文件到 S3
func (s *S3Storage) SaveFile(file *multipart.FileHeader, savePath string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(savePath),
		Body:        src,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("上传到S3失败: %w", err)
	}
	return nil
}

// DeleteFile 从 S3 删除文件
func (s *S3Storage) DeleteFile(fileCode *models.FileCodes) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(fileCode.GetFilePath()),
	})
	if err != nil {
		return fmt.Errorf("从S3删除文件失败: %w", err)
	}
	return nil
}

// GetFileURL 获取服务器中转下载URL
func (s *S3Storage) GetFileURL(fileCode *models.FileCodes) string {
	return utils.GetFileURL(fileCode.Code, "")
}

// GetFileResponse S3 存储通过生成预签名 URL 流式下载
func (s *S3Storage) GetFileResponse(fileCode *models.FileCodes) (string, error) {
	// S3 文件不在本地，返回空路径 + 特殊标记，由 handler 通过 presigned URL 流式传输
	return "", fmt.Errorf("S3文件需通过流式代理下载: %s", fileCode.GetFilePath())
}

// SaveChunk 保存分片到 S3
func (s *S3Storage) SaveChunk(uploadID string, chunkIndex int, chunkData []byte, chunkHash string, savePath string) error {
	chunkKey := filepath.ToSlash(filepath.Join(filepath.Dir(savePath), "chunks", uploadID, fmt.Sprintf("%d.part", chunkIndex)))

	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(chunkKey),
		Body:   bytesReader(chunkData),
	})
	return err
}

// MergeChunks 合并 S3 上的分片（通过 Multipart Upload Copy）
func (s *S3Storage) MergeChunks(uploadID string, chunkInfo *models.UploadChunk, savePath string) (string, string, error) {
	chunkBaseDir := filepath.ToSlash(filepath.Join(filepath.Dir(savePath), "chunks", uploadID))

	// 初始化 Multipart Upload
	createResp, err := s.client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(savePath),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", "", fmt.Errorf("创建MultipartUpload失败: %w", err)
	}
	uploadID2 := *createResp.UploadId

	var parts []types.CompletedPart
	fileHasher := sha256.New()

	for i := 0; i < chunkInfo.TotalChunks; i++ {
		chunkKey := fmt.Sprintf("%s/%d.part", chunkBaseDir, i)

		// 下载分片数据做哈希
		getResp, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    aws.String(chunkKey),
		})
		if err != nil {
			s.client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(s.bucketName),
				Key:      aws.String(savePath),
				UploadId: aws.String(uploadID2),
			})
			return "", "", fmt.Errorf("读取分片%d失败: %w", i, err)
		}
		chunkData, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		fileHasher.Write(chunkData)

		// UploadPartCopy
		copyResp, err := s.client.UploadPartCopy(context.TODO(), &s3.UploadPartCopyInput{
			Bucket:     aws.String(s.bucketName),
			Key:        aws.String(savePath),
			UploadId:   aws.String(uploadID2),
			PartNumber: aws.Int32(int32(i + 1)),
			CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucketName, chunkKey)),
		})
		if err != nil {
			s.client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(s.bucketName),
				Key:      aws.String(savePath),
				UploadId: aws.String(uploadID2),
			})
			return "", "", fmt.Errorf("复制分片%d失败: %w", i, err)
		}

		parts = append(parts, types.CompletedPart{
			PartNumber: aws.Int32(int32(i + 1)),
			ETag:       copyResp.CopyPartResult.ETag,
		})
	}

	// 完成 Multipart Upload
	_, err = s.client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucketName),
		Key:      aws.String(savePath),
		UploadId: aws.String(uploadID2),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("完成MultipartUpload失败: %w", err)
	}

	fileHash := fmt.Sprintf("%x", fileHasher.Sum(nil))
	return savePath, fileHash, nil
}

// GeneratePresignedUploadURL 生成 S3 预签名上传 URL
func (s *S3Storage) GeneratePresignedUploadURL(savePath string, expiresIn int) (string, error) {
	req, err := s.presignClient.PresignPutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(savePath),
	}, func(po *s3.PresignOptions) {
		po.Expires = time.Duration(expiresIn) * time.Second
	})
	if err != nil {
		return "", fmt.Errorf("生成预签名URL失败: %w", err)
	}
	return req.URL, nil
}

// FileExists 检查 S3 文件是否存在
func (s *S3Storage) FileExists(savePath string) (bool, error) {
	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(savePath),
	})
	if err != nil {
		return false, nil // 不存在或错误都视为 false
	}
	return true, nil
}

// CleanChunks 清理 S3 上的临时分片
func (s *S3Storage) CleanChunks(uploadID string, savePath string) error {
	chunkBaseDir := filepath.ToSlash(filepath.Join(filepath.Dir(savePath), "chunks", uploadID))

	// 列出并删除所有分片对象
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(chunkBaseDir + "/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			slog.Info("列出S3分片失败", "prefix", chunkBaseDir, "error", err)
			return nil
		}
		for _, obj := range page.Contents {
			s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucketName),
				Key:    obj.Key,
			})
		}
	}
	return nil
}

// bytesReader 将 []byte 包装成 io.Reader
type bytesReaderCloser struct {
	data   []byte
	offset int
}

func (r *bytesReaderCloser) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func bytesReader(data []byte) io.ReadSeeker {
	// S3 SDK 需要 io.ReadSeeker，我们用临时文件
	f, err := os.CreateTemp("", "s3chunk-*.part")
	if err != nil {
		// 回退：用 strings.NewReader 不支持 seek
		return &nopSeeker{data: data}
	}
	f.Write(data)
	f.Seek(0, 0)
	// 注册清理
	fname := f.Name()
	// 简单的 defer remove 更好
	return &tmpFileReader{file: f, name: fname}
}

type nopSeeker struct {
	data   []byte
	offset int
}

func (r *nopSeeker) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func (r *nopSeeker) Seek(offset int64, whence int) (int64, error) {
	// 简化实现
	if whence == io.SeekStart {
		r.offset = int(offset)
	} else if whence == io.SeekCurrent {
		r.offset += int(offset)
	}
	return int64(r.offset), nil
}

type tmpFileReader struct {
	file *os.File
	name string
}

func (r *tmpFileReader) Read(p []byte) (int, error) { return r.file.Read(p) }
func (r *tmpFileReader) Seek(offset int64, whence int) (int64, error) {
	return r.file.Seek(offset, whence)
}

// Close 实现 io.Closer，删除临时文件
func (r *tmpFileReader) Close() error {
	r.file.Close()
	os.Remove(r.name)
	return nil
}

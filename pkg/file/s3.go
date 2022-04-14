package file

import (
	"context"
	"github.com/minio/minio-go/v6"
)

type s3Uploader struct {
	Endpoint        string
	AccessKeyId     string
	SecretAccessKey string
}

func NewS3Uploader(Endpoint, AK, SK string) *s3Uploader {
	return &s3Uploader{
		Endpoint:        Endpoint,
		AccessKeyId:     AK,
		SecretAccessKey: SK,
	}
}

func (su *s3Uploader) InitClient() (*minio.Client, error) {
	return minio.New(su.Endpoint, su.AccessKeyId, su.SecretAccessKey, true)
}

func (su *s3Uploader) Upload(ctx context.Context, filePath, bucketName, objectName string) (int64, error) {
	minioClient, err := su.InitClient()
	if err != nil {
		return 0, err
	}
	// bucketname
	// http://docs.minio.org.cn/docs/master/golang-client-quickstart-guide
	// 需要自己创建 bucket  这里创建的名为 dfyio
	//bucketName := "dfyio"
	//objectName := "etcd-sanpshot.db"
	Size, err := minioClient.FPutObject(bucketName, objectName, filePath, minio.PutObjectOptions{})
	if err != nil {
		return 0, err
	}
	return Size, nil
}

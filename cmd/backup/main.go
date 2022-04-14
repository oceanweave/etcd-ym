package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/oceanweave/etcd-ym/api/v1alpha1"
	"github.com/oceanweave/etcd-ym/pkg/file"
	clientv3 "go.etcd.io/etcd/client/v3"
	//"go.etcd.io/etcd/snapshot"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

func logErr(log logr.Logger, err error, message string) error {
	log.Error(err, message)
	return fmt.Errorf("%s: %s", message, err)
}

func main() {
	var (
		backupTempDir      string
		etcdURL            string
		backupURL          string
		dialTimeoutSeconds int64
		timeoutSeconds     int64
	)

	// os.TempDir() 创建一个 tmp 文件夹 tmp/
	flag.StringVar(&backupTempDir, "backup-tmp-dir", os.TempDir(), "The diectory to temp place backup etcd cluster.")
	flag.StringVar(&etcdURL, "etcd-url", "", "URL for backup etcd.")
	flag.StringVar(&backupURL, "backup-url", "", "URL for backup etcd object storage.")
	flag.Int64Var(&dialTimeoutSeconds, "dial-timeout-seconds", 5, "Timeout for dialing the Etcd.")
	flag.Int64Var(&timeoutSeconds, "timeout-seconds", 60, "Timeout for Backup the Etcd.")
	// 忘记解析了 一定要加上
	flag.Parse()

	zapLogger := zap.NewRaw(zap.UseDevMode(true))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeoutSeconds))
	defer ctxCancel()

	log := ctrl.Log.WithName("backup").WithValues("etcd-url", etcdURL)

	log.Info("Connecting to Etcd and getting Snapshot data")
	// 定义一个本地的数据目录go
	localPath := filepath.Join(backupTempDir, "snapshot.db")
	// 创建 etcd snapshot manager
	etcdManager := snapshot.NewV3(zapLogger)
	// 保存 etcd snapshot 数据到 localPath
	err := etcdManager.Save(ctx, clientv3.Config{
		Endpoints:   []string{etcdURL},
		DialTimeout: time.Second * time.Duration(dialTimeoutSeconds),
	}, localPath)
	if err != nil {
		panic(logErr(log, err, "failed to get etcd snapshot data"))
	}

	// 数据保存到本地成功
	// 接下来就上传
	// http://docs.minio.org.cn/docs/master/golang-client-quickstart-guide
	// todo, 根据传递进来的参数判断初始化是 s3 还是 oss

	storageType, bucketName, objectName, err := file.ParseBackupURL(backupURL)
	if err != nil {
		panic(logErr(log, err, "failed to parse backup storage path"))
	}
	switch storageType {
	case string(v1alpha1.BackupStorageTypeS3):
		log.Info("Uploading snapshot...")
		// 上传 etcd snapshot 到 远程存储 s3
		size, err := handleS3(ctx, bucketName, objectName, localPath)
		if err != nil {
			panic(logErr(log, err, "failed to upload backup etcd"))
		}
		log.WithValues("upload-size", size).Info("Backup Completed")
	case string(v1alpha1.BackupStorageTypeOSS):
	default:
		panic(logErr(log, fmt.Errorf("storage type error"), fmt.Sprintf("unknown stprage tuype:%v", storageType)))
	}

}

func handleS3(ctx context.Context, bucketName, objectName, localPath string) (int64, error) {
	//endpoint := "play.min.io"
	//accessKeyID := "Q3AM3UQ867SPQQA43P2F"
	//secretAccessKey := "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
	endpoint := os.Getenv("ENDPOINT")
	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET")
	s3Uploader := file.NewS3Uploader(endpoint, accessKeyID, secretAccessKey)
	size, err := s3Uploader.Upload(ctx, localPath, bucketName, objectName)
	return size, err
}

/*
Copyright 2022 dfy.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
var (
	EtcdBackupPhaseBackingUp EtcdBackupPhase = "BackingUp"
	EtcdBackupPhaseCompleted EtcdBackupPhase = "Completed"
	EtcdBackupPhaseFailed    EtcdBackupPhase = "Failed"

	BackupStorageTypeS3  BackupStorageType = "s3"
	BackupStorageTypeOSS BackupStorageType = "oss"
)

// 为什么重命名？ 因为易读 具有含义
type BackupStorageType string
type EtcdBackupPhase string

// EtcdBackupSpec defines the desired state of EtcdBackup
type EtcdBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// etcd集群是一个分布式系统，由多个节点相互通信构成整体的对外服务，
	// 每个节点都存储了完整的数据，并且通过Raft协议保证了每个节点维护的数据都是一致的。
	// https://zhuanlan.zhihu.com/p/87014600
	// 因此只需要备份一个节点即可
	EtcdUrl      string            `json:"etcdUrl"`
	StorageType  BackupStorageType `json:"storageType"`
	BackupSource `json:",inline"`
}

type BackupSource struct {
	S3  *S3BackupSource  `json:"s3,omitempty"`
	OSS *OSSBackupSource `json:"oss,omitempty"`
}

type S3BackupSource struct {
	Path     string `json:"path"`
	Endpoint string `json:"endpoint"`
	// Secret Object: AcessKey AccessSecret
	Secret string `json:"secret"`
}

type OSSBackupSource struct {
	Path string `json:"path"`
	// Secret Object: AcessKey AccessSecret
	OSSSecret string `json:"ossSecret"`
}

// EtcdBackupStatus defines the observed state of EtcdBackup
type EtcdBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase          EtcdBackupPhase `json:"phase"`
	StartTime      *metav1.Time    `json:"startTime,omitempty"`
	ComPletionTime *metav1.Time    `json:"completionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EtcdBackup is the Schema for the etcdbackups API
type EtcdBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdBackupSpec   `json:"spec,omitempty"`
	Status EtcdBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EtcdBackupList contains a list of EtcdBackup
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdBackup{}, &EtcdBackupList{})
}

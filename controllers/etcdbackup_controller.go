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

package controllers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/oceanweave/etcd-ym/api/v1alpha1"
)

// EtcdBackupReconciler reconciles a EtcdBackup object
type EtcdBackupReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	BackupImage string
}

type backupState struct {
	backup  *etcdv1alpha1.EtcdBackup // backup yaml 就是此 CRD
	actual  *backupStateContainer    // 获取当前 备份 pod 的状态
	desired *backupStateContainer    // 利用 CRD 中的信息 构建期望备份pod的状态
}

// 就是个 pod 只不过封装一下
type backupStateContainer struct {
	pod *corev1.Pod
}

// 获取当前应用的整个状态
func (r *EtcdBackupReconciler) getState(ctx context.Context, req ctrl.Request) (*backupState, error) {
	var state backupState

	// 获取 EtcdBackup 对象
	state.backup = &etcdv1alpha1.EtcdBackup{}
	if err := r.Get(ctx, req.NamespacedName, state.backup); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("getting backup object error: %s", err)
		}
		// 被删除了 直接忽略
		state.backup = nil
		return &state, nil
	}
	// 获得了 EtcdBackup 对象
	// 获取当前真实的状态
	if err := r.setStateActual(ctx, &state); err != nil {
		return nil, fmt.Errorf("getting actual state error: %s", err)
	}
	// 获取期望的状态
	if err := r.setStateDesired(&state); err != nil {
		return nil, fmt.Errorf("getting desired state error: %s", err)
	}
	return &state, nil
}

// 获取真实的状态
func (r *EtcdBackupReconciler) setStateActual(ctx context.Context, state *backupState) error {
	var actual backupStateContainer
	key := client.ObjectKey{
		Name:      state.backup.Name,
		Namespace: state.backup.Namespace,
	}
	actual.pod = &corev1.Pod{}
	if err := r.Get(ctx, key, actual.pod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("getting pod error: %s", err)
		}
		// NotFound 错误，被删除了 直接忽略
		actual.pod = nil
	}

	// 填充当前真实的状态
	state.actual = &actual
	return nil
}

func (r *EtcdBackupReconciler) setStateDesired(state *backupState) error {
	var desired backupStateContainer
	// 根据 EtcdBackup 创建一个用于备份 etcd 的 Pod
	// 创建 pod
	pod := podforBackup(state.backup, r.BackupImage)

	// 配置 controller references
	if err := controllerutil.SetControllerReference(state.backup, pod, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference error: %s", err)
	}
	desired.pod = pod
	// 获取到期望的对象
	state.desired = &desired
	return nil
}

// 根据要求 构建 pod 的yaml
// 备份所使用的 pod
func podforBackup(backup *etcdv1alpha1.EtcdBackup, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "etcd-backup",
					Image: image,
					Args: []string{ // image 的 参数
						"--etcd-url", backup.Spec.EtcdUrl,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patchd
//+kubebuilder:rbac:groups=etcd.dfy.io,resources=etcdbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=etcd.dfy.io,resources=etcdbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=etcd.dfy.io,resources=etcdbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EtcdBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *EtcdBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log2 := log.FromContext(ctx, "etcdbackup", req.NamespacedName)

	// TODO(user): your logic here
	// 获取 backupState
	state, err := r.getState(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 根据状态来判断下一步要执行的动作
	var action Action

	// 开始判断状态
	switch {
	case state.backup == nil: // 此CRD 被删除了
		log2.Info("Backup Object not found")
	case !state.backup.DeletionTimestamp.IsZero(): // 被标记为删除
		log2.Info("Backup Object has been deleted")
	case state.backup.Status.Phase == "": // 要开始备份了，先标记状态为备份中
		log2.Info("Backup starting...")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdBackupPhaseBackingUp // 更新状态
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}

	case state.actual.pod == nil: // 当前还么有执行任务的 Pod
		log2.Info("Backup Pod does not exists. Creating...")
		action = &CreateObject{client: r.Client, obj: state.desired.pod} // 创建
	case state.actual.pod.Status.Phase == corev1.PodFailed: // Pod 执行失败
		log2.Info("Backup Pod Failed.")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdBackupPhaseFailed // 更改成备份失败
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}
	case state.actual.pod.Status.Phase == corev1.PodSucceeded: // Pod 执行成功
		log2.Info("Backup Pod Failed.")
		newBackup := state.backup.DeepCopy()
		newBackup.Status.Phase = etcdv1alpha1.EtcdBackupPhaseCompleted // 更改成备份成功
		action = &PatchStatus{client: r.Client, original: state.backup, new: newBackup}
	// 目前成功了 就是记录一下日志
	case state.backup.Status.Phase == etcdv1alpha1.EtcdBackupPhaseFailed: // 失败了
		log2.Info("Backup has failed. Ignoring...")
	case state.backup.Status.Phase == etcdv1alpha1.EtcdBackupPhaseCompleted: // 成功了
		log2.Info("Backup has completed. Ignoring...")
	}

	if action != nil {
		if err := action.Execute(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdBackup{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

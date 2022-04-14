package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Action 所有情况下都 需要执行的动作
// 有个问题，情况的种类很多，那怎么调用一个方法就可以呢？
// 因此就将不同种类封装为 不同的结构体，当需要执行时，就构建相应的结构体，进行执行
type Action interface {
	Execute(ctx context.Context) error
}

type PatchStatus struct {
	client   client.Client
	original client.Object
	new      client.Object
}

// Execute 更新 Status
func (s *PatchStatus) Execute(ctx context.Context) error {
	if reflect.DeepEqual(s.original, s.new) {
		return nil
	}
	// 更新状态 将 new merge 合并到 原本状态上
	if err := s.client.Status().Patch(ctx, s.new, client.MergeFrom(s.original)); err != nil {
		return fmt.Errorf("patching status error: %s", err)
	}
	return nil
}

// CreateObject 创建一个新的资源对象
type CreateObject struct {
	client client.Client
	obj    client.Object
}

func (o *CreateObject) Execute(ctx context.Context) error {
	if err := o.client.Create(ctx, o.obj); err != nil {
		return fmt.Errorf("create object error: %s", err)
	}
	return nil
}

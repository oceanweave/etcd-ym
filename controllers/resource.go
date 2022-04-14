package controllers

import (
	etcdv1alpha1 "github.com/oceanweave/etcd-ym/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

var (
	EtcdClusterLabelKey      = "dfy.io/cluster"
	EtcdCluterCommonLabelKey = "app"
	EtcdDataDirName          = "datadir"
)

// metadata:
//  name: etcd
//  labels:
//    app: etcd
//spec:
//  ports:
//  - port: 2380
//    name: etcd-server
//  - port: 2379
//    name: etcd-client
//  clusterIP: None
//  selector:
//    app: etcd
//  publishNotReadyAddresses: true
// 根据 EtcdCluster 组装 Headless Service
func MutateHeadlessService(cluster *etcdv1alpha1.EtcdCluster, service *corev1.Service) {
	// 给 service 添加 通用 label
	// Service 的 Label
	service.Labels = map[string]string{
		EtcdCluterCommonLabelKey: "etcd",
	}
	// 构建 Service spec
	service.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		// Service 的 选择器
		Selector: map[string]string{
			EtcdClusterLabelKey: cluster.Name,
		},
		Ports: []corev1.ServicePort{
			corev1.ServicePort{
				Name: "peer",
				Port: 2380, // 端口也可以不写死，从 cluster 获取
			},
			corev1.ServicePort{
				Name: "client",
				Port: 2379,
			},
		},
	}
}

// 根据 EtcdCluster 组装 StatefulSet
func MutateStatefulSet(cluster *etcdv1alpha1.EtcdCluster, sts *appsv1.StatefulSet) {
	sts.Labels = map[string]string{
		EtcdCluterCommonLabelKey: "etcd",
	}
	sts.Spec = appsv1.StatefulSetSpec{
		Replicas:    cluster.Spec.Size,
		ServiceName: cluster.Name,
		// Statefule Set 的 选择器
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				EtcdClusterLabelKey: cluster.Name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				// Pod 的 Label  注意要对应上
				Labels: map[string]string{
					EtcdClusterLabelKey:      cluster.Name,
					EtcdCluterCommonLabelKey: "etcd",
				},
			},
			Spec: corev1.PodSpec{
				Containers: newContainers(cluster), // 构建容器 调用下面的函数
			},
		},
		//  volumeClaimTemplates:
		//  - metadata:
		//      name: datadir
		//    spec:
		//      accessModes:
		//      - "ReadWriteOnce"
		//      resources:
		//        requests:
		//          # upstream recommended max is 700M
		//          storage: 1Gi
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					// 与上面 Pod 定义的存储路径关联
					Name: EtcdDataDirName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
}

func newContainers(cluster *etcdv1alpha1.EtcdCluster) []corev1.Container {
	return []corev1.Container{
		corev1.Container{
			Name:            "etcd",
			Image:           cluster.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				corev1.ContainerPort{
					Name:          "peer",
					ContainerPort: 2380,
				},
				corev1.ContainerPort{
					Name:          "client",
					ContainerPort: 2379,
				},
			},
			Env: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "INITIAL_CLUSTER_SIZE",
					Value: strconv.Itoa(int(*cluster.Spec.Size)),
				},
				corev1.EnvVar{
					Name:  "SET_NAME",
					Value: cluster.Name,
				},
				corev1.EnvVar{
					Name: "MY_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
				corev1.EnvVar{
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				corev1.VolumeMount{
					// Pod 中 容器的存储路径，与 pvc 模板名字相同
					Name:      EtcdDataDirName,
					MountPath: "/var/run/etcd",
				},
			},
			Command: []string{
				"/bin/sh",
				"-ec",
				"HOSTNAME=$(hostname)\n\n              ETCDCTL_API=3\n\n              eps() {\n                  EPS=\"\"\n                  " +
					"for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                      EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379\"\n                  done\n                  echo ${EPS}\n              }\n\n              " +
					"member_hash() {\n                  etcdctl member list | grep -w \"$HOSTNAME\" | awk '{ print $1}' | awk -F \",\" '{ print $1}'\n              }\n\n              " +
					"initial_peers() {\n                  PEERS=\"\"\n                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                    " +
					"PEERS=\"${PEERS}${PEERS:+,}${SET_NAME}-${i}=http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380\"\n                  done\n                  echo ${PEERS}\n              }\n\n              " +
					"# etcd-SET_ID\n              SET_ID=${HOSTNAME##*-}\n\n              # adding a new member to existing cluster (assuming all initial pods are available)\n              " +
					"if [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n                  # export ETCDCTL_ENDPOINTS=$(eps)\n                  # member already added?\n\n                  MEMBER_HASH=$(member_hash)\n                  if [ -n \"${MEMBER_HASH}\" ]; then\n                      " +
					"# the member hash exists but for some reason etcd failed\n                      # as the datadir has not be created, we can remove the member\n                      # and retrieve new hash\n                      echo \"Remove member ${MEMBER_HASH}\"\n                      etcdctl --endpoints=$(eps) member remove ${MEMBER_HASH}\n                  fi\n\n                 " +
					" echo \"Adding new member\"\n\n                  echo \"etcdctl --endpoints=$(eps) member add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380\"\n                  etcdctl member --endpoints=$(eps) add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 | grep \"^ETCD_\" > /var/run/etcd/new_member_envs\n\n                  if [ $? -ne 0 ]; then\n                      " +
					"echo \"member add ${HOSTNAME} error.\"\n                      rm -f /var/run/etcd/new_member_envs\n                      exit 1\n                  fi\n\n                  echo \"==> Loading env vars of existing cluster...\"\n                  sed -ie \"s/^/export /\" /var/run/etcd/new_member_envs\n                  cat /var/run/etcd/new_member_envs\n                  . /var/run/etcd/new_member_envs\n\n                  " +
					"echo \"etcd --name ${HOSTNAME} --initial-advertise-peer-urls ${ETCD_INITIAL_ADVERTISE_PEER_URLS} --listen-peer-urls http://${POD_IP}:2380 --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 --data-dir /var/run/etcd/default.etcd --initial-cluster ${ETCD_INITIAL_CLUSTER} --initial-cluster-state ${ETCD_INITIAL_CLUSTER_STATE}\"\n\n                  " +
					"exec etcd --listen-peer-urls http://${POD_IP}:2380 \\\n                      --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n                      --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \\\n                      --data-dir /var/run/etcd/default.etcd\n              fi\n\n              for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                  " +
					"while true; do\n                      echo \"Waiting for ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local to come up\"\n                      ping -W 1 -c 1 ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local > /dev/null && break\n                      sleep 1s\n                  done\n              done\n\n              echo \"join member ${HOSTNAME}\"\n              # join member\n              exec etcd --name ${HOSTNAME} \\\n                  " +
					"--initial-advertise-peer-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 \\\n                  --listen-peer-urls http://${POD_IP}:2380 \\\n                  --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n                  --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \\\n                  --initial-cluster-token etcd-cluster-1 \\\n                  --data-dir /var/run/etcd/default.etcd \\\n                  " +
					"--initial-cluster $(initial_peers) \\\n                  --initial-cluster-state new",
			},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"/bin/sh",
							"-ec",
							"HOSTNAME=$(hostname)\n\n                    member_hash() {\n                        etcdctl member list | grep -w \"$HOSTNAME\" | awk '{ print $1}' | awk -F \",\" '{ print $1}'\n                    }\n\n                    eps() {\n                        EPS=\"\"\n                        for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                            EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379\"\n                        done\n                        echo ${EPS}\n                    }\n\n                    export ETCDCTL_ENDPOINTS=$(eps)\n                    SET_ID=${HOSTNAME##*-}\n\n                    # Removing member from cluster\n                    if [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n                        echo \"Removing ${HOSTNAME} from etcd cluster\"\n                        etcdctl member remove $(member_hash)\n                        if [ $? -eq 0 ]; then\n                            # Remove everything otherwise the cluster will no longer scale-up\n                            rm -rf /var/run/etcd/*\n                        fi\n                    fi",
						},
					},
				},
			},
		},
	}
}

// metadata:
//  labels:
//    app: etcd
//  name: etcd
//spec:
//  replicas: 3
//  selector:
//    matchLabels:
//      app: etcd
//  serviceName: etcd
//  template:
//    metadata:
//      labels:
//        app: etcd
//    spec:
//      containers:
//        - name: etcd
//          image: cnych/etcd:v3.4.13
//          imagePullPolicy: IfNotPresent
//          ports:
//          - containerPort: 2380
//            name: peer
//            protocol: TCP
//          - containerPort: 2379
//            name: client
//            protocol: TCP
//          env:
//          - name: INITIAL_CLUSTER_SIZE
//            value: "3"
//          - name: MY_NAMESPACE
//            valueFrom:
//              fieldRef:
//                fieldPath: metadata.namespace
//          - name: POD_IP
//            valueFrom:
//              fieldRef:
//                fieldPath: status.podIP
//          - name: SET_NAME
//            value: "etcd"
//          command:
//            - /bin/sh
//            - -ec
//            - |
//              HOSTNAME=$(hostname)
//
//              ETCDCTL_API=3
//
//              eps() {
//                  EPS=""
//                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
//                      EPS="${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379"
//                  done
//                  echo ${EPS}
//              }
//
//              member_hash() {
//                  etcdctl member list | grep -w "$HOSTNAME" | awk '{ print $1}' | awk -F "," '{ print $1}'
//              }
//
//              initial_peers() {
//                  PEERS=""
//                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
//                    PEERS="${PEERS}${PEERS:+,}${SET_NAME}-${i}=http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380"
//                  done
//                  echo ${PEERS}
//              }
//
//              # etcd-SET_ID
//              SET_ID=${HOSTNAME##*-}
//
//              # adding a new member to existing cluster (assuming all initial pods are available)
//              if [ "${SET_ID}" -ge ${INITIAL_CLUSTER_SIZE} ]; then
//                  # export ETCDCTL_ENDPOINTS=$(eps)
//                  # member already added?
//
//                  MEMBER_HASH=$(member_hash)
//                  if [ -n "${MEMBER_HASH}" ]; then
//                      # the member hash exists but for some reason etcd failed
//                      # as the datadir has not be created, we can remove the member
//                      # and retrieve new hash
//                      echo "Remove member ${MEMBER_HASH}"
//                      etcdctl --endpoints=$(eps) member remove ${MEMBER_HASH}
//                  fi
//
//                  echo "Adding new member"
//
//                  echo "etcdctl --endpoints=$(eps) member add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380"
//                  etcdctl member --endpoints=$(eps) add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 | grep "^ETCD_" > /var/run/etcd/new_member_envs
//
//                  if [ $? -ne 0 ]; then
//                      echo "member add ${HOSTNAME} error."
//                      rm -f /var/run/etcd/new_member_envs
//                      exit 1
//                  fi
//
//                  echo "==> Loading env vars of existing cluster..."
//                  sed -ie "s/^/export /" /var/run/etcd/new_member_envs
//                  cat /var/run/etcd/new_member_envs
//                  . /var/run/etcd/new_member_envs
//
//                  echo "etcd --name ${HOSTNAME} --initial-advertise-peer-urls ${ETCD_INITIAL_ADVERTISE_PEER_URLS} --listen-peer-urls http://${POD_IP}:2380 --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 --data-dir /var/run/etcd/default.etcd --initial-cluster ${ETCD_INITIAL_CLUSTER} --initial-cluster-state ${ETCD_INITIAL_CLUSTER_STATE}"
//
//                  exec etcd --listen-peer-urls http://${POD_IP}:2380 \
//                      --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \
//                      --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \
//                      --data-dir /var/run/etcd/default.etcd
//              fi
//
//              for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
//                  while true; do
//                      echo "Waiting for ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local to come up"
//                      ping -W 1 -c 1 ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local > /dev/null && break
//                      sleep 1s
//                  done
//              done
//
//              echo "join member ${HOSTNAME}"
//              # join member
//              exec etcd --name ${HOSTNAME} \
//                  --initial-advertise-peer-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 \
//                  --listen-peer-urls http://${POD_IP}:2380 \
//                  --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \
//                  --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \
//                  --initial-cluster-token etcd-cluster-1 \
//                  --data-dir /var/run/etcd/default.etcd \
//                  --initial-cluster $(initial_peers) \
//                  --initial-cluster-state new
//          lifecycle:
//            preStop:
//              exec:
//                command:
//                  - /bin/sh
//                  - -ec
//                  - |
//                    HOSTNAME=$(hostname)
//
//                    member_hash() {
//                        etcdctl member list | grep -w "$HOSTNAME" | awk '{ print $1}' | awk -F "," '{ print $1}'
//                    }
//
//                    eps() {
//                        EPS=""
//                        for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
//                            EPS="${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379"
//                        done
//                        echo ${EPS}
//                    }
//
//                    export ETCDCTL_ENDPOINTS=$(eps)
//                    SET_ID=${HOSTNAME##*-}
//
//                    # Removing member from cluster
//                    if [ "${SET_ID}" -ge ${INITIAL_CLUSTER_SIZE} ]; then
//                        echo "Removing ${HOSTNAME} from etcd cluster"
//                        etcdctl member remove $(member_hash)
//                        if [ $? -eq 0 ]; then
//                            # Remove everything otherwise the cluster will no longer scale-up
//                            rm -rf /var/run/etcd/*
//                        fi
//                    fi
//          volumeMounts:
//          - mountPath: /var/run/etcd
//            name: datadir
//  volumeClaimTemplates:
//  - metadata:
//      name: datadir
//    spec:
//      accessModes:
//      - "ReadWriteOnce"
//      resources:
//        requests:
//          # upstream recommended max is 700M
//          storage: 1Gi

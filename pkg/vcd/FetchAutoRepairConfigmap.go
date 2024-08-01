package vcd

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func GetRebootingPrivilege(kubeclient kubernetes.Interface) bool {
	var privilege string
	configmaps, err := kubeclient.CoreV1().ConfigMaps("kube-system").Get(context.Background(), "auto-repair-configmap", metav1.GetOptions{})
	if err != nil {
		fmt.Println("cannot get information from auto-repair configmap")
		klog.Fatalf("Failed to get information of auto-repair configmap: %v", err)
	}
	privilege = configmaps.Data["enable_replacing_node"]
	enableRebooting, _ := strconv.ParseBool(privilege)
	return enableRebooting
}

func GetReplacingPrivilege(kubeclient kubernetes.Interface) bool {
	var privilege string
	configmaps, err := kubeclient.CoreV1().ConfigMaps("kube-system").Get(context.Background(), "auto-repair-configmap", metav1.GetOptions{})
	if err != nil {
		fmt.Println("cannot get information from auto-repair configmap")
		klog.Fatalf("Failed to get information of auto-repair configmap: %v", err)
	}
	privilege = configmaps.Data["enable_replacing_node"]
	enableReplacing, _ := strconv.ParseBool(privilege)
	return enableReplacing
}

func GetWaitingTimeForNotReadyNode(kubeclient kubernetes.Interface) int {
	var waitingTime string
	configmaps, err := kubeclient.CoreV1().ConfigMaps("kube-system").Get(context.Background(), "auto-repair-configmap", metav1.GetOptions{})
	if err != nil {
		fmt.Println("cannot get information from auto-repair configmap")
		klog.Fatalf("Failed to get information of auto-repair configmap: %v", err)
	}
	waitingTime = configmaps.Data["waiting_time_for_not_ready_node"]
	waitingTimeForNotReadyNode, _ := strconv.Atoi(waitingTime)
	return waitingTimeForNotReadyNode
}
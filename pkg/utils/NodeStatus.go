package utils

import (
	ctx "context"
	"fmt"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	logger "k8s.io/klog/v2"
	"time"
)
// return true if node in notReady state
func GetNodeStatus(node *core.Node) bool {
    for _, condition := range node.Status.Conditions {
        if condition.Type == core.NodeReady && condition.Status != core.ConditionTrue {
            return true
        }
    }
	return false
}
//checking status of node continuously
func CheckNodeReadyStatusAfterRepairing(node *core.Node, clientSet kubernetes.Interface) bool {
	logger.Info("Rebooted node in infrastructure, waiting for Ready state in kubernetes")
	maxRetry := 10
	retryDuration := 15 * time.Second
	//retryCount := 0
	for retryCount := 0; retryCount < maxRetry; retryCount++ {
		newNodeState, err := clientSet.CoreV1().Nodes().Get(ctx.Background(), node.Name, metav1.GetOptions{}) 
		if err != nil {
			logger.Info("node not found")
			return false
		}
		for _, condition := range newNodeState.Status.Conditions {
        	if condition.Type == core.NodeReady && condition.Status == core.ConditionTrue {
				logger.Info("node is healthy now", "node", node.Name)
				fmt.Println("node is healthy now")
				time.Sleep(retryDuration)
            	return true
			}
	}
	logger.Info("can not determine if node is healthy, retry after 10 seconds")
	time.Sleep(retryDuration)
	}
	return false
}
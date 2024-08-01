package utils

import (
	"context"
	ctx "context"
	"errors"
	"fmt"
	"strconv"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	logger "k8s.io/klog/v2"
)

func AddAnnotationForNode(clientSet kubernetes.Interface , node *core.Node, annotationKey string, annotationValue string) error {
	currentNodeStatus, nodeNotFoundErr := clientSet.CoreV1().Nodes().Get(ctx.Background(), node.Name, metav1.GetOptions{}) 
	if nodeNotFoundErr != nil {
		return errors.New("node not found")
	}

	currentNodeStatus.Annotations[annotationKey] = annotationValue
	_, err := clientSet.CoreV1().Nodes().Patch(ctx.Background(), currentNodeStatus.Name, types.MergePatchType, []byte(fmt.Sprintf(`{"metadata": {"annotations": {"%s": "%s"}}}`, annotationKey, annotationValue)), metav1.PatchOptions{})
	if err != nil {
		logger.Info("can not assign annotation for node")
		panic(err)
	}
	return nil
}

func RemoveAnnotation(clientSet kubernetes.Interface , node *core.Node, annotationKey string) error {
	currentNodeStatus, nodeNotFoundErr := clientSet.CoreV1().Nodes().Get(ctx.Background(), node.Name, metav1.GetOptions{}) 
	if nodeNotFoundErr != nil {
		return errors.New("node not found")
	}

	delete(currentNodeStatus.Annotations, annotationKey)
	_, err := clientSet.CoreV1().Nodes().Update(context.Background(), currentNodeStatus, metav1.UpdateOptions{})
	if err != nil {
		logger.Info("can not remove annotation for this node")
		panic(err)
	}
	logger.Info("removed  annotation for this node")
	return nil
}

func GetAnnotation(clientSet kubernetes.Interface , node *core.Node, annotationKey string) (string, error) {
	currentNodeStatus, nodeNotFoundErr := clientSet.CoreV1().Nodes().Get(ctx.Background(), node.Name, metav1.GetOptions{}) 
	if nodeNotFoundErr != nil {
		return "", errors.New("node not found")
	}

	nodeAnnotation, ok := currentNodeStatus.Annotations[annotationKey]

	if !ok {
		return "", errors.New("can not get annotation")
	}

	return nodeAnnotation, nil

}

func CheckTotalNumberOfRebooting(node *core.Node) int {
	annotationKey := "TotalNumberOfRebootingByAutoRepair"
	nodeAnnotation, ok := node.Annotations[annotationKey]

	if !ok {
		logger.Info("can not get annotation from this node")
		return 0
	}
	totalNumberOfRebooting,_ := strconv.Atoi(nodeAnnotation)
	return totalNumberOfRebooting
}
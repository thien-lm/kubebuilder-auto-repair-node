/*
Copyright 2024.

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

package controller

import (
	"context"
	pureError "errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"github.com/thien-lm/node-autorepair/pkg/draino"
	"github.com/thien-lm/node-autorepair/pkg/utils"
	"github.com/thien-lm/node-autorepair/pkg/vcd"
	"github.com/thien-lm/node-autorepair/pkg/vcd/manipulation"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clientset *kubernetes.Clientset
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Node object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

    node := &corev1.Node{}
    err := r.Get(ctx, req.NamespacedName, node)
    if err != nil {
        if errors.IsNotFound(err) {
            // Node not found, return without error
            logger.Info("Node not found")
            return ctrl.Result{}, nil
        }
        // Error reading the object
        logger.Error(err, "Failed to get Node")
        return ctrl.Result{}, err
    }
	// node is master node, do nothing
	if strings.Contains(node.Name, "master") {
		// logger.Info("master node will not be handled by this controller")
		return ctrl.Result{}, nil
	}
	// if node status is fine, just remove it out of queue and remove annotation 
	if !utils.GetNodeStatus(node) {
		if _,err := utils.GetAnnotation(r.Clientset, node, "RebootingSolvedIssue"); err == nil {
			_ = utils.RemoveAnnotation(r.Clientset, node, "RebootingSolvedIssue")
		}
		if _,err := utils.GetAnnotation(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair"); err == nil {
			_ = utils.RemoveAnnotation(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair")
		}
		logger.Info("Node status is OK, no need to be handled")
		return ctrl.Result{}, nil
	} else {
		logger.Info("-----Reconciling node object-----\n")
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				lastTransitionTime := condition.LastTransitionTime.Time
				if time.Since(lastTransitionTime) <= 3*time.Minute  { 
					logger.Info("node was in not ready state, recheck after 3 minutes")
					return ctrl.Result{RequeueAfter: 3*time.Minute}, nil
				}
			}
		}
	}
	//TODO: if node in not ready state but not more than 3 minutes, add to queue to process again
	
	// if node is not ready, first reboot node if capable
	totalNumberOfRebooting :=  utils.CheckTotalNumberOfRebooting(node)
	if vcd.GetRebootingPrivilege(r.Clientset) && totalNumberOfRebooting <= 2 {
		logger.Info("trying to reboot node: ", "node" , node.Name)
		//fetch info to access to vmware platform
		goVcloudClient, org, vdc, err := vcd.CreateGoVCloudClient(r.Clientset)

		if err != nil {
			logger.Error(pureError.New("failed to connect"), "failed to connect to vcd")
			//if the total number times of rebooting is > 3, does not reboot node anymore
			// if totalNumberOfRebooting == 0 {
			// 	utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", "1")
			// } else if totalNumberOfRebooting <= 2 {
			// 	utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", strconv.Itoa(totalNumberOfRebooting + 1))
			// 	// if the node was not rebooted more than 3 times, re-enqueue it to process again
			// 	return ctrl.Result{RequeueAfter: 10*time.Minute}, err
			// } 
			return ctrl.Result{RequeueAfter: 10*time.Minute}, err
		}
		//reboot vm in infra
		err2 := manipulation.RebootVM(goVcloudClient, org, vdc, node.Name)
		if err2 != nil {
			logger.Error(pureError.New("failed to reboot"), "failed to reboot node")
			// if totalNumberOfRebooting == 0 {
			// 	utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", "1")
			// } else if totalNumberOfRebooting <= 2 {
			// 	utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", strconv.Itoa(totalNumberOfRebooting + 1))
			// 	// if the node was not rebooted more than 3 times, re-enqueue it to process again
			// 	return ctrl.Result{RequeueAfter: 5*time.Minute}, err
			// } 
			return ctrl.Result{RequeueAfter: 5*time.Minute}, err
		}

		isNodeReady := utils.CheckNodeReadyStatusAfterRepairing(node, r.Clientset)
		if isNodeReady {
			logger.Info("repair node perform by auto repair controller was ran successfully")
			return ctrl.Result{}, nil
		} else {
			if totalNumberOfRebooting == 0 {
				utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", "1")
				logger.Info("rebooted, node will be re enqueue and reprocessed after 10 minutes")
				return ctrl.Result{RequeueAfter: 5*time.Minute}, err
			} else if totalNumberOfRebooting <= 2 {
				utils.AddAnnotationForNode(r.Clientset, node, "TotalNumberOfRebootingByAutoRepair", strconv.Itoa(totalNumberOfRebooting + 1))
				// if the node was not rebooted more than 3 times, re-enqueue it to process again
				logger.Info("rebooted, node will be re enqueue and reprocessed after 10 minutes")
				return ctrl.Result{RequeueAfter: 5*time.Minute}, err
			} 			
		}
	}

	//if rebooting does not solve node issue, drain that node 
	ActorDeleteNode, err := utils.GetAnnotation(r.Clientset, node, "ActorDeleteNode")
	// if err != nil {
	// 	ActorDeleteNode = "ClusterAutoScaler"
	// }
	if vcd.GetReplacingPrivilege(r.Clientset) && ActorDeleteNode != "ClusterAutoScaler" && ActorDeleteNode != "NodeAutoRepair"{
		log, _ := zap.NewProduction()
		condition := []string{"Ready"}
		pf := []draino.PodFilterFunc{draino.MirrorPodFilter}
		// pf = append(pf, draino.LocalStoragePodFilter)
		// pf = append(pf, draino.UnreplicatedPodFilter)
		// pf = append(pf, draino.NewDaemonSetPodFilter(r.Clientset))
		// pf = append(pf, draino.NewStatefulSetPodFilter(r.Clientset))
		
		newDrainer := draino.NewDrainingResourceEventHandler(r.Clientset,
			draino.NewAPICordonDrainer(r.Clientset,
				draino.MaxGracePeriod(5*time.Minute),
				draino.EvictionHeadroom(30*time.Second),
				draino.WithSkipDrain(false),
				draino.WithPodFilter(draino.NewPodFilters(pf...)),
				draino.WithAPICordonDrainerLogger(log),
			),
			draino.NewEventRecorder(r.Clientset),
			draino.WithLogger(log),
			draino.WithDrainBuffer(10*time.Minute),
			draino.WithConditionsFilter(condition))
		utils.AddAnnotationForNode(r.Clientset, node, "ActorDeleteNode", "ClusterAutoScaler")
		newDrainer.HandleNode(node)
		logger.Info("Waiting for node to be deleted by Cluster Auto Scaler, re-check this node after 10 minutes")
		return ctrl.Result{RequeueAfter: 10*time.Minute}, nil
	}

	// if node can not be resolved by cluster autoscaler,remove it manually by manual scale down API
	AutoRepairStatus, err := utils.GetAnnotation(r.Clientset, node, "AutoRepairStatus")
	if vcd.GetReplacingPrivilege(r.Clientset) && ActorDeleteNode == "ClusterAutoScaler" && (AutoRepairStatus != "NodeAutoRepairFailedToResolveNode" && err != nil) {
		// maxAutoScalerNode := vcd.GetMaxSize(r.Clientset)
		// minAutoScalerNode := vcd.GetMinSize(r.Clientset)
		accessToken := vcd.GetAccessToken(r.Clientset)
		vpcID := vcd.GetVPCId(r.Clientset)
		callbackURL := vcd.GetCallBackURL(r.Clientset)
		domainAPI := utils.GetDomainApiConformEnv(callbackURL)
		clusterIDPortal := utils.GetClusterID(r.Clientset)
		idCluster := utils.GetIDCluster(domainAPI, vpcID, accessToken, clusterIDPortal)
		// fmt.Println("access token: ", accessToken, "vpc: ", vpcID, "callbackURL", callbackURL, "domainAPI", domainAPI, "clusteridPortal", clusterIDPortal , "id cluster", idCluster)
		//if it is not out of timeout or status on portal is scaling, re enqueue it to check later
		if !utils.CheckStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) || ActorDeleteNode == "NodeAutoRepair" {
			logger.Info("Waiting for node to be deleted by Cluster Auto Scaler(sometime Node Auto Repair), re-check this node after 10 minutes")
			return ctrl.Result{RequeueAfter: 5*time.Minute}, nil
		}
		//if node auto-repair was disabled, just delete drained node, then cluster autoscaler will scale up cluster again, the portal need to be in SUCCESS state
		// if maxAutoScalerNode == minAutoScalerNode && utils.CheckSucceedStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {
		// 	utils.AddAnnotationForNode(r.Clientset, node, "ActorDeleteNode", "NodeAutoRepair")
		// 	status, _ := utils.ScaleDown(time.Now() , r.Clientset, accessToken, vpcID, idCluster, clusterIDPortal, callbackURL, node.Name)
		// 	time.Sleep(20 * time.Second)
		// 	if !status {
		// 		// node was removed
		// 		err := utils.AddAnnotationForNode(r.Clientset, node, "AutoRepairStatus", "NodeAutoRepairFailedToResolveNode")
		// 		if err != nil {
		// 			logger.Info("node was not in cluster anymore")
		// 			return ctrl.Result{}, nil
		// 		}
		// 	}
		// 	if status && utils.CheckErrorStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {
		// 		_ = utils.AddAnnotationForNode(r.Clientset, node, "AutoRepairStatus", "NodeAutoRepairFailedToResolveNode")
		// 		//forget the event => het cuu node
		// 		logger.Info("Failed to handle this node")
		// 		return ctrl.Result{}, nil
		// 	} 	
		// }
		
		// if cluster autoscaler failed to replace node, remove it manually, the portal will be in ERROR state
		if  utils.CheckErrorStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {
			logger.Info("the node will be deleted by node auto repair")
			utils.AddAnnotationForNode(r.Clientset, node, "ActorDeleteNode", "NodeAutoRepair")
			status, _ := utils.ScaleDown(time.Now() , r.Clientset, accessToken, vpcID, idCluster, clusterIDPortal, callbackURL, node.Name)
			time.Sleep(20 * time.Second)
			if status && utils.CheckErrorStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {
				err := utils.AddAnnotationForNode(r.Clientset, node, "AutoRepairStatus", "NodeAutoRepairFailedToResolveNode")
				if err != nil {
					logger.Info("node was not in cluster anymore")
					return ctrl.Result{}, nil
				}
				//forget the event => het cuu node
				logger.Info("Failed to handle this node")
				return ctrl.Result{}, nil
			} 	
		}
	}
	//het cuu node
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // Create a clientset from the manager's config
    config := ctrl.GetConfigOrDie()
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return fmt.Errorf("failed to create clientset: %w", err)
    }
    r.Clientset = clientset

    return ctrl.NewControllerManagedBy(mgr).
        For(&corev1.Node{}).
        WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
        Complete(r)
}


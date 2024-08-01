package utils
import (
	"time"
	kube_client "k8s.io/client-go/kubernetes"
	logger "k8s.io/klog/v2"
)

func ScaleDown(currentTime time.Time, kubeclient kube_client.Interface,
	accessToken, vpcID, idCluster, clusterIDPortal, callbackURL, workerNameToRemove  string) (bool, error) {
	domainAPI := GetDomainApiConformEnv(callbackURL)
	workerNodeNameList := make([]string, 0)
	workerNodeNameList = append(workerNodeNameList, workerNameToRemove) //draft
	// if status of cluster is not equal scaling, do nothing
	if CheckStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {

		PerformScaleDown(domainAPI, vpcID, accessToken, idCluster, clusterIDPortal, workerNodeNameList) // draft
		for {
			var count int = 0
			time.Sleep(30 * time.Second)
			isSucceededStatus := CheckSucceedStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal)
			logger.Info("Status of cluster is SCALING, checking if cluster scale down successfully after 30 seconds")
			count = count + 1
			if isSucceededStatus {
				logger.Info("Status of cluster is SUCCEEDED")
				break
			}
			if count > 100 {
				break //break if timeout (50 minutes)
			}
			isErrorStatus := CheckErrorStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal)
			if isErrorStatus {
				PerformScaleDown(domainAPI, vpcID, accessToken, idCluster, clusterIDPortal, workerNodeNameList)
				for {
					time.Sleep(30 * time.Second)
					if CheckStatusCluster(domainAPI, vpcID, accessToken, clusterIDPortal) {
						break
					}
				}
				break
			}
		}
	} else {

		logger.Info("Another action is being performed")
		logger.Info("Waiting for scaling ...")
		return false, nil
	}

	logger.Info("End of scale down process")
	return true, nil
}
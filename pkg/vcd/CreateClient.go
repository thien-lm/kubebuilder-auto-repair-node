package vcd

import (
    "fmt"
    "net/url"
    "github.com/vmware/go-vcloud-director/v2/govcd"
    "k8s.io/client-go/kubernetes"
)

type Config struct {
    User     string
    Password string
    Org      string
    Href     string
    VDC      string
    Insecure bool
}

func (c *Config) NewClient() (*govcd.VCDClient, error) {
    u, err := url.ParseRequestURI(c.Href)
    if err != nil {
        return nil, fmt.Errorf("unable to pass url: %s", err)
    }
    vcdclient := govcd.NewVCDClient(*u, true)
    err = vcdclient.Authenticate(c.User, c.Password, c.Org)
    if err != nil {
        return nil, fmt.Errorf("unable to authenticate: %s", err)
    }
    return vcdclient, nil
}

func CreateGoVCloudClient(kubeclient kubernetes.Interface) (*govcd.VCDClient, string, string, error) {
	kubeClient := kubeclient
	userName := GetUserName(kubeClient)
	password := GetPassword(kubeClient)
	host     := GetHost(kubeClient)
	org 	:= GetORG(kubeClient)
	vdc     := GetVDC(kubeClient)

	goVCloudClientConfig := Config{
		User: userName,
		Password: password,
		Href: fmt.Sprintf("%v/api", host),
		Org: org,
		VDC: vdc,
	}
	
	goVcloudClient, err := goVCloudClientConfig.NewClient()
	return goVcloudClient, org, vdc ,err 
}
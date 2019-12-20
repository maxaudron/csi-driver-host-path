package client

import (
	"fmt"
	"sync"

	zfsvolumev1 "github.com/maxaudron/zfs-csi-driver/pkg/apis/zfsvolume/v1"
	zfsvolumev1apisclientset "github.com/maxaudron/zfs-csi-driver/pkg/client/clientset/versioned"

	"github.com/maxaudron/zfs-csi-driver/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	PatchJSONTypeReplace string = "replace"
	PatchJSONTypeAdd     string = "add"
)

var (
	onceDefaultzfsvolumeV1Client sync.Once
	defaultClient                *Client
	validPatchResources          map[string]string
)

// Client is an API client to help perform CRUD for CRD instances.
type Client struct {
	clientset *zfsvolumev1apisclientset.Clientset
	namespace string
	plural    string
}

// PatchJSONTypeOps describes the operations for PATCH defined in RFC6902. https://tools.ietf.org/html/rfc6902
// The supported operations are: add, remove, replace, move, copy and test.
// When we news a zfsvolume instance, we'll set defatule value for all fields. So, when you want to patch a zfsvolume,
// DO NOT use remove. Please use replace, even if you want to keey that field "empty".
// Example:
// 	things := make([]IntThingSpec, 2)
// 	things[0].Op = "replace"
// 	things[0].Path = "/status/message"
// 	things[0].Value = "1234"
// 	things[1].Op = "replace"
// 	things[1].Path = "/status/state"
// 	things[1].Value = ""
type PatchJSONTypeOps struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// GetNamespace returns the namespace the client talks to.
func (c *Client) GetNamespace() string {
	return c.namespace
}

// GetPlural returns the plural the client is managing.
func (c *Client) GetPlural() string {
	return c.plural
}

// CreatezfsvolumeClientset returns the clientset for CRD zfsvolume v1 in singleton way.
func CreateZFSVolumeClientset(kubeconfigPath string) (*zfsvolumev1apisclientset.Clientset, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := zfsvolumev1apisclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// NewClient accepts kubeconfig path and namespace. Return the API client interface for CRD zfsvolume v1.
func NewClient(kubeconfigPath string) (*Client, error) {
	clientset, err := CreateZFSVolumeClientset(kubeconfigPath)
	if err != nil {
		fmt.Printf("Fail to init CRD API clientset for Jinghuazhu v1: %+v\n", err.Error())

		return nil, err
	}
	c := &Client{
		clientset: clientset,
		plural:    zfsvolumev1.Plural,
	}

	return c, nil
}

// GetDefaultClient returns an API client interface for CRD zfsvolume v1. It assumes the kubeconfig
// is available at default path and the target CRD namespace is the default namespace.
func GetDefaultClient() *Client {
	onceDefaultzfsvolumeV1Client.Do(func() {
		clientset, err := CreateZFSVolumeClientset(types.DefaultKubeConfigPath)
		if err != nil {
			panic("Fail to init default CRD API client for Jinghuazhu v1: " + err.Error())
		}
		defaultClient = &Client{
			clientset: clientset,
			plural:    zfsvolumev1.Plural,
		}
	})

	return defaultClient
}

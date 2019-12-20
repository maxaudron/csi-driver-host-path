package client

import (
	"encoding/json"

	zfsvolumev1 "github.com/maxaudron/zfs-csi-driver/pkg/apis/zfsvolume/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
)

// Create post an instance of CRD into Kubernetes.
func (c *Client) Create(obj *zfsvolumev1.ZFSVolume) (*zfsvolumev1.ZFSVolume, error) {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Create(obj)
}

// Update puts new instance of CRD to replace the old one.
func (c *Client) Update(obj *zfsvolumev1.ZFSVolume) (*zfsvolumev1.ZFSVolume, error) {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Update(obj)
}

// Patch applies the patch and returns the patched zfsvolume v1 instance.
func (c *Client) Patch(name string, pt apimachinerytypes.PatchType, data []byte, subresources ...string) (*zfsvolumev1.ZFSVolume, error) {
	var result zfsvolumev1.ZFSVolume
	err := c.clientset.RESTClient().Patch(pt).
		Namespace(c.namespace).
		Resource(c.plural).
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(&result)

	return &result, err
}

// PatchJSONType uses JSON Type (RFC6902) in PATCH.
func (c *Client) PatchJSONType(name string, ops []PatchJSONTypeOps) (*zfsvolumev1.ZFSVolume, error) {
	patchBytes, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}

	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Patch(name, apimachinerytypes.JSONPatchType, patchBytes)
}

// PatchSpec only updates the spec field of zfsvolume v1, which is /spec.
func (c *Client) PatchSpec(name string, zfsvolumeSpec *zfsvolumev1.ZFSVolumeSpec) (*zfsvolumev1.ZFSVolume, error) {
	ops := make([]PatchJSONTypeOps, 1, 1)
	ops[0].Op = PatchJSONTypeReplace
	ops[0].Path = "/spec"
	ops[0].Value = zfsvolumeSpec

	return c.PatchJSONType(name, ops)
}

// Delete removes the CRD instance by given name and delete options.
func (c *Client) Delete(name string, opts *metav1.DeleteOptions) error {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Delete(name, opts)
}

// Get returns a pointer to the CRD instance.
func (c *Client) Get(name string, opts metav1.GetOptions) (*zfsvolumev1.ZFSVolume, error) {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Get(name, opts)
}

// GetWithoutOps retrieves the zfsvolume instance without any GetOptions.
func (c *Client) GetWithoutOps(name string) (*zfsvolumev1.ZFSVolume, error) {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).Get(name, metav1.GetOptions{})
}

// List returns a list of CRD instances by given list options.
func (c *Client) List(opts metav1.ListOptions) (*zfsvolumev1.ZFSVolumeList, error) {
	return c.clientset.ZfsV1().ZFSVolumes(c.namespace).List(opts)
}

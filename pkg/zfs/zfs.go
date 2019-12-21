/*
Copyright 2017 The Kubernetes Authors.

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

package zfs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	// "k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	zfsvolumev1 "github.com/maxaudron/zfs-csi-driver/pkg/apis/zfsvolume/v1"
	zfsvolumev1client "github.com/maxaudron/zfs-csi-driver/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilexec "k8s.io/utils/exec"

	timestamp "github.com/golang/protobuf/ptypes/timestamp"
)

const (
	kib    int64 = 1024
	mib    int64 = kib * 1024
	gib    int64 = mib * 1024
	gib100 int64 = gib * 100
	tib    int64 = gib * 1024
	tib100 int64 = tib * 100
)

type hostPath struct {
	name              string
	nodeID            string
	version           string
	endpoint          string
	ephemeral         bool
	maxVolumesPerNode int64

	ids *identityServer
	ns  *nodeServer
	cs  *controllerServer
}

type hostPathVolume struct {
	VolName        string `json:"volName"`
	VolID          string `json:"volID"`
	VolSize        int64  `json:"volSize"`
	VolPath        string `json:"volPath"`
	ZFSPool        string `json:"zfsPool"`
	ZFSCompression string `json:"zfsCompression"`
	ZFSDedup       string `json:"zfsDedup"`
	Ephemeral      bool   `json:"ephemeral"`
}

type hostPathSnapshot struct {
	Name         string              `json:"name"`
	Id           string              `json:"id"`
	VolID        string              `json:"volID"`
	Path         string              `json:"path"`
	CreationTime timestamp.Timestamp `json:"creationTime"`
	SizeBytes    int64               `json:"sizeBytes"`
	ReadyToUse   bool                `json:"readyToUse"`
}

var (
	vendorVersion = "dev"

	zfsVolumeClient *zfsvolumev1client.Client

	hostPathVolumeSnapshots map[string]hostPathSnapshot
)

// zfs related constants
const (
	ZFS_DEVPATH = "/dev/zvol/"
	FSTYPE_ZFS  = "zfs"
)

// zfs command related constants
const (
	ZFSVolCmd     = "zfs"
	ZFSCreateArg  = "create"
	ZFSDestroyArg = "destroy"
	ZFSSetArg     = "set"
	ZFSListArg    = "list"
)

// constants to define volume type
const (
	VOLTYPE_DATASET = "DATASET"
	VOLTYPE_ZVOL    = "ZVOL"
)

const (
	// Directory where data for volumes and snapshots are persisted.
	// This can be ephemeral within the container or persisted if
	// backed by a Pod volume.
	dataRoot = "/csi-data-dir"
)

func init() {
	hostPathVolumeSnapshots = map[string]hostPathSnapshot{}
}

func NewHostPathDriver(driverName, nodeID, endpoint string, ephemeral bool, maxVolumesPerNode int64, version string) (*hostPath, error) {
	if driverName == "" {
		return nil, fmt.Errorf("No driver name provided")
	}

	if nodeID == "" {
		return nil, fmt.Errorf("No node id provided")
	}

	if endpoint == "" {
		return nil, fmt.Errorf("No driver endpoint provided")
	}
	if version != "" {
		vendorVersion = version
	}

	glog.Infof("Driver: %v ", driverName)
	glog.Infof("Version: %s", vendorVersion)

	kubeConfigPath := os.Getenv("KUBECONFIG")

	// Create a CRD client interface for Jinghzhu v1.
	var err error
	zfsVolumeClient, err = zfsvolumev1client.NewClient(kubeConfigPath)
	if err != nil {
		panic(err)
	}

	return &hostPath{
		name:              driverName,
		version:           vendorVersion,
		nodeID:            nodeID,
		endpoint:          endpoint,
		ephemeral:         ephemeral,
		maxVolumesPerNode: maxVolumesPerNode,
	}, nil
}

func (hp *hostPath) Run() {
	// Create GRPC servers
	hp.ids = NewIdentityServer(hp.name, hp.version)
	hp.ns = NewNodeServer(hp.nodeID, hp.ephemeral, hp.maxVolumesPerNode)
	hp.cs = NewControllerServer(hp.ephemeral, hp.nodeID)

	s := NewNonBlockingGRPCServer()
	s.Start(hp.endpoint, hp.ids, hp.cs, hp.ns)
	s.Wait()
}

func getVolumeByID(volumeID string) (*zfsvolumev1.ZFSVolume, error) {
	vols, err := zfsVolumeClient.List(metav1.ListOptions{})
	if err != nil {
		return &zfsvolumev1.ZFSVolume{}, fmt.Errorf("Error while retriving volumes: %s: %s", volumeID, err)
	}

	for _, vol := range vols.Items {
		if vol.Spec.ID == volumeID {
			return &vol, nil
		}
	}
	return &zfsvolumev1.ZFSVolume{}, fmt.Errorf("Could not find volume: %s", volumeID)
}

func getVolumeByName(volumeName string) (*zfsvolumev1.ZFSVolume, error) {
	if hostPathVol, err := zfsVolumeClient.Get(volumeName, metav1.GetOptions{}); err == nil {
		return hostPathVol, nil
	} else {
		return &zfsvolumev1.ZFSVolume{}, fmt.Errorf("Error while retriving volume: %s: %s", volumeName, err)
	}
}

func getSnapshotByName(name string) (hostPathSnapshot, error) {
	for _, snapshot := range hostPathVolumeSnapshots {
		if snapshot.Name == name {
			return snapshot, nil
		}
	}
	return hostPathSnapshot{}, fmt.Errorf("snapshot name %s does not exit in the snapshots list", name)
}

// getVolumePath returs the canonical path for zfs volume
func getVolumePath(volID string) string {
	vol, err := getVolumeByID(volID)
	if err != nil {
		panic(err)
	}
	return vol.Spec.Path
}

// builldDatasetCreateArgs returns zfs create command for dataset along with attributes as a string array
func buildDatasetCreateArgs(vol *zfsvolumev1.ZFSVolume) []string {
	var ZFSVolArg []string

	volume := vol.Spec.Pool + "/" + vol.ObjectMeta.Name

	ZFSVolArg = append(ZFSVolArg, ZFSCreateArg)

	quotaProperty := "quota=" + strconv.FormatInt(vol.Spec.Size, 10)
	ZFSVolArg = append(ZFSVolArg, "-o", quotaProperty)

	if len(vol.Spec.Dedup) != 0 {
		dedupProperty := "dedup=" + vol.Spec.Dedup
		ZFSVolArg = append(ZFSVolArg, "-o", dedupProperty)
	}
	if len(vol.Spec.Compression) != 0 {
		compressionProperty := "compression=" + vol.Spec.Compression
		ZFSVolArg = append(ZFSVolArg, "-o", compressionProperty)
	}

	ZFSVolArg = append(ZFSVolArg, volume)

	return ZFSVolArg
}

// createVolume create the directory for the zfs volume.
// It returns the volume path or err if one occurs.
func createHostpathVolume(volID, name string, cap int64, compression string, dedup string, pool string, ephemeral bool) (*zfsvolumev1.ZFSVolume, error) {
	zfsVolSpec := zfsvolumev1.ZFSVolumeSpec{
		ID:          volID,
		Size:        cap,
		Path:        filepath.Join("/", filepath.Join(pool, name)),
		Pool:        pool,
		Compression: compression,
		Dedup:       dedup,
	}
	zfsVolMeta := metav1.ObjectMeta{
		Name: name,
	}
	zfsVol := zfsvolumev1.ZFSVolume{
		ObjectMeta: zfsVolMeta,
		Spec:       zfsVolSpec,
	}

	cmd := exec.Command(ZFSVolCmd, buildDatasetCreateArgs(&zfsVol)...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to allocate volume %v: %v: %s", zfsVolSpec.ID, err, out)
	}

	vol, err := zfsVolumeClient.Create(&zfsVol)
	return vol, err
}

// updateVolume updates the existing zfs volume.
func updateHostpathVolume(volID string, volume *zfsvolumev1.ZFSVolume) error {
	glog.V(4).Infof("updating zfs volume: %s", volID)

	if _, err := getVolumeByID(volID); err != nil {
		return err
	}

	_, err := zfsVolumeClient.Update(volume)
	return err
}

// builldVolumeDestroyArgs returns volume destroy command along with attributes as a string array
func buildVolumeDestroyArgs(vol *zfsvolumev1.ZFSVolume) []string {
	var ZFSVolArg []string

	volume := vol.Spec.Pool + "/" + vol.ObjectMeta.Name

	ZFSVolArg = append(ZFSVolArg, ZFSDestroyArg, "-R", volume)

	return ZFSVolArg
}

// deleteVolume deletes the directory for the zfs volume.
func deleteHostpathVolume(volID string) error {
	glog.V(4).Infof("deleting zfs volume: %s", volID)

	vol, err := getVolumeByID(volID)
	if err != nil {
		// Return OK if the volume is not found.
		return nil
	}

	cmd := exec.Command(ZFSVolCmd, buildVolumeDestroyArgs(vol)...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		return status.Errorf(codes.Internal, "failed to delete volume %v: %v: %s", vol.Spec.ID, err, out)
	}

	err = zfsVolumeClient.Delete(vol.ObjectMeta.Name, &metav1.DeleteOptions{})
	return err
}

// hostPathIsEmpty is a simple check to determine if the specified zfs directory
// is empty or not.
func hostPathIsEmpty(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return true, fmt.Errorf("unable to open zfs volume, error: %v", err)
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// TODO rewrite for zfs snapshots
// loadFromSnapshot populates the given destPath with data from the snapshotID
func loadFromSnapshot(snapshotId, destPath string) error {
	snapshot, ok := hostPathVolumeSnapshots[snapshotId]
	if !ok {
		return status.Errorf(codes.NotFound, "cannot find snapshot %v", snapshotId)
	}
	if snapshot.ReadyToUse != true {
		return status.Errorf(codes.Internal, "snapshot %v is not yet ready to use.", snapshotId)
	}
	snapshotPath := snapshot.Path
	args := []string{"zxvf", snapshotPath, "-C", destPath}
	executor := utilexec.New()
	out, err := executor.Command("tar", args...).CombinedOutput()
	if err != nil {
		return status.Errorf(codes.Internal, "failed pre-populate data from snapshot %v: %v: %s", snapshotId, err, out)
	}
	return nil
}

// loadfromVolume populates the given destPath with data from the srcVolumeID
func loadFromVolume(srcVolumeId, destPath string) error {
	hostPathVolume, err := zfsVolumeClient.Get(srcVolumeId, metav1.GetOptions{})
	if err != nil {
		return status.Error(codes.NotFound, "source volumeId does not exist, are source/destination in the same storage class?")
	}
	srcPath := hostPathVolume.Spec.Path
	isEmpty, err := hostPathIsEmpty(srcPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed verification check of source zfs volume: %s: %v", srcVolumeId, err)
	}

	// If the source zfs volume is empty it's a noop and we just move along, otherwise the cp call will fail with a a file stat error DNE
	if !isEmpty {
		args := []string{"-a", srcPath + "/.", destPath + "/"}
		executor := utilexec.New()
		out, err := executor.Command("cp", args...).CombinedOutput()
		if err != nil {
			return status.Errorf(codes.Internal, "failed pre-populate data from volume %v: %v: %s", srcVolumeId, err, out)
		}
	}
	return nil
}

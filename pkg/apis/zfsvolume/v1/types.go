package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ZFSVolume is a top-level type
type ZFSVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// This is where you can define
	// your own custom spec
	Spec ZFSVolumeSpec `json:"spec,omitempty"`
}

// custom spec
type ZFSVolumeSpec struct {
	ID          string `json:"id,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Path        string `json:"path,omitempty"`
	Pool        string `json:"pool,omitempty"`
	Compression string `json:"compression,omitempty"`
	Dedup       string `json:"dedup,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// no client needed for list as it's been created in above
type ZFSVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `son:"metadata,omitempty"`

	Items []ZFSVolume `json:"items"`
}

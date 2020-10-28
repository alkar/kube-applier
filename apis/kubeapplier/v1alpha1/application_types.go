package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// DryRun enables the dry-run flag when applying this application.
	// +optional
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun,omitempty"`

	// Prune determines whether pruning is enabled for this application.
	// +optional
	// +kubebuilder:default=true
	Prune bool `json:"prune,omitempty"`

	// PruneClusterResources determines whether pruning is enabled for cluster
	// resources, as part of this Application.
	// +optional
	// +kubebuilder:default=false
	PruneClusterResources bool `json:"pruneClusterResources,omitempty"`

	// PruneBlacklist can be used to specify a list of resources that are exempt
	// from pruning.
	// +optional
	PruneBlacklist []string `json:"pruneBlacklist,omitempty"`

	// RepositoryName defines a Repository resource from which configuration is
	// pulled for this Application.
	// +optional
	RepositoryName string `json:"repositoryName,omitempty"`

	// RepositoryPath defines the relative path inside the Repository where the
	// configuration for this Application is stored.
	RepositoryPath string `json:"repositoryPath"`

	// RunInterval determines how often this application is applied in seconds.
	// +optional
	// +kubebuilder:default=3600
	RunInterval int `json:"runInterval,omitempty"`

	// ServerSideApply determines whether the server-side apply flag is enabled
	// for this Application.
	// +optional
	// +kubebuilder:default=false
	ServerSideApply bool `json:"serverSideApply,omitempty"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	// LastRun contains the last apply run's information.
	// +nullable
	// +optional
	LastRun *ApplicationStatusRun `json:"lastRun,omitempty"`
}

// ApplicationStatusRun contains information about an apply run of an
// Application resource.
type ApplicationStatusRun struct {
	// Commit is the git commit hash on which this apply run operated.
	Commit string `json:"commit"`

	// Finished is the time that the apply run finished.
	Finished metav1.Time `json:"finished"`

	// Started is the time that the apply run started.
	Started metav1.Time `json:"started"`

	// Success denotes whether the apply run was successful or not.
	Success bool `json:"success"`

	// Type is a short description of the kind of apply run that was attempted.
	// +kubebuilder:default="unknown"
	Type string `json:"type"`
}

// +kubebuilder:object:root=true

// Application is the Schema for the Applications API of kube-applier. An
// Application is defined as a namespace associated with a path in a remote git
// repository where kubernetes configuration is stored.
// +kubebuilder:resource:shortName=app;apps
// +kubebuilder:subresource:status
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}

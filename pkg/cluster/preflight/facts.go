package preflight

// Facts is everything the preflight reads from the cluster. Collection is read-only and
// happens once (see collector.go); the checks in checks.go are pure functions over Facts so
// they can be unit tested without a cluster. Every section carries the error that prevented
// its collection, so a check can tell "absent" from "could not read" and never turns missing
// evidence into a PASS.
type Facts struct {
	Version        VersionFacts
	Access         AccessFacts
	Nodes          NodeList
	QoveryNS       NamespaceFacts
	HelmReleases   HelmReleaseList
	CRDs           CRDList
	Deployments    DeploymentList
	StorageClasses StorageClassList
	CSIDrivers     NameList
	ClusterDNS     ClusterDNSFacts
	LoadBalancers  LoadBalancerList
	MetricsAPI     MetricsAPIFacts
}

type VersionFacts struct {
	Raw   string
	Major int
	Minor int
	Err   error
}

type AccessFacts struct {
	// ClusterAdmin is true when a SelfSubjectAccessReview allows verb "*" on every resource of
	// every API group, cluster-wide.
	ClusterAdmin bool
	Err          error
}

type Node struct {
	Name          string
	Ready         bool
	Unschedulable bool
	OS            string
	Arch          string
	ProviderID    string
	Labels        map[string]string
	CPUMilli      int64
	MemoryBytes   int64
}

type NodeList struct {
	Items []Node
	Err   error
}

type NamespaceFacts struct {
	Exists      bool
	Terminating bool
	Err         error
}

type HelmRelease struct {
	Name   string
	Status string
}

type HelmReleaseList struct {
	// Items are the Helm releases stored in the qovery namespace, read from the labels of the
	// Helm storage Secrets only (metadata, never Secret data).
	Items []HelmRelease
	Err   error
}

type ObjectMeta struct {
	Namespace   string
	Name        string
	Labels      map[string]string
	Annotations map[string]string
}

type CRDList struct {
	Items map[string]ObjectMeta
	Err   error
}

type DeploymentList struct {
	Items []ObjectMeta
	Err   error
}

type StorageClass struct {
	Name        string
	Default     bool
	Provisioner string
}

type StorageClassList struct {
	Items []StorageClass
	Err   error
}

type NameList struct {
	Items []string
	Err   error
}

type ClusterDNSFacts struct {
	ServiceFound   bool
	ReadyEndpoints int
	Err            error
}

type LoadBalancer struct {
	Namespace  string
	Name       string
	HasAddress bool
}

type LoadBalancerList struct {
	Items []LoadBalancer
	Err   error
}

type MetricsAPIFacts struct {
	Available bool
	Err       error
}

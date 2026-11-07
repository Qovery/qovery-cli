package preflight

import (
	"context"
	"sort"
	"strconv"
	"strings"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

var (
	secretsResource     = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	deploymentsResource = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	crdsResource        = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
)

// Collect reads the facts the checks need. It only issues get, list and
// SelfSubjectAccessReview requests: it never creates, changes or deletes a cluster object, and
// it reads Secrets and Deployments through the metadata API, so no Secret data is fetched.
func Collect(ctx context.Context, config *rest.Config) (Facts, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return Facts{}, err
	}
	meta, err := metadata.NewForConfig(config)
	if err != nil {
		return Facts{}, err
	}
	return collectWith(ctx, clientset, meta), nil
}

func collectWith(ctx context.Context, clientset kubernetes.Interface, meta metadata.Interface) Facts {
	f := Facts{}
	f.Version = collectVersion(clientset)
	if f.Version.Err != nil {
		// Without the API server, every other read fails the same way.
		return f
	}
	f.Access = collectAccess(ctx, clientset)
	f.Nodes = collectNodes(ctx, clientset)
	f.QoveryNS = collectQoveryNamespace(ctx, clientset)
	f.HelmReleases = collectHelmReleases(ctx, meta, f.QoveryNS)
	f.CRDs = collectCRDs(ctx, meta, crdCertManager, crdEnvoyProxy, crdGatewayAPI)
	f.Deployments = collectDeployments(ctx, meta)
	f.StorageClasses = collectStorageClasses(ctx, clientset)
	f.CSIDrivers = collectCSIDrivers(ctx, clientset)
	f.ClusterDNS = collectClusterDNS(ctx, clientset)
	f.LoadBalancers = collectLoadBalancers(ctx, clientset)
	f.MetricsAPI = collectMetricsAPI(clientset)
	return f
}

func leadingInt(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

func collectVersion(c kubernetes.Interface) VersionFacts {
	info, err := c.Discovery().ServerVersion()
	if err != nil {
		return VersionFacts{Err: err}
	}
	// Managed providers report minors such as "31+".
	return VersionFacts{Raw: info.GitVersion, Major: leadingInt(info.Major), Minor: leadingInt(info.Minor)}
}

func collectAccess(ctx context.Context, c kubernetes.Interface) AccessFacts {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{Verb: "*", Group: "*", Resource: "*"},
		},
	}
	res, err := c.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return AccessFacts{Err: err}
	}
	return AccessFacts{ClusterAdmin: res.Status.Allowed}
}

func collectNodes(ctx context.Context, c kubernetes.Interface) NodeList {
	list, err := c.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return NodeList{Err: err}
	}
	out := NodeList{}
	for _, n := range list.Items {
		node := Node{
			Name:          n.Name,
			Unschedulable: n.Spec.Unschedulable,
			OS:            n.Status.NodeInfo.OperatingSystem,
			Arch:          n.Status.NodeInfo.Architecture,
			ProviderID:    n.Spec.ProviderID,
			Labels:        n.Labels,
		}
		if node.OS == "" {
			node.OS = n.Labels[corev1.LabelOSStable]
		}
		if node.Arch == "" {
			node.Arch = n.Labels[corev1.LabelArchStable]
		}
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				node.Ready = true
			}
		}
		if cpu, ok := n.Status.Allocatable[corev1.ResourceCPU]; ok {
			node.CPUMilli = cpu.MilliValue()
		}
		if mem, ok := n.Status.Allocatable[corev1.ResourceMemory]; ok {
			node.MemoryBytes = mem.Value()
		}
		out.Items = append(out.Items, node)
	}
	return out
}

func collectQoveryNamespace(ctx context.Context, c kubernetes.Interface) NamespaceFacts {
	ns, err := c.CoreV1().Namespaces().Get(ctx, qoveryNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return NamespaceFacts{}
	}
	if err != nil {
		return NamespaceFacts{Err: err}
	}
	return NamespaceFacts{
		Exists:      true,
		Terminating: ns.Status.Phase == corev1.NamespaceTerminating || ns.DeletionTimestamp != nil,
	}
}

// collectHelmReleases lists the Helm releases of namespace qovery from the labels Helm puts on
// its storage Secrets (owner=helm, name, status, version), keeping the latest revision.
func collectHelmReleases(ctx context.Context, meta metadata.Interface, ns NamespaceFacts) HelmReleaseList {
	if ns.Err == nil && !ns.Exists {
		return HelmReleaseList{}
	}
	list, err := meta.Resource(secretsResource).Namespace(qoveryNamespace).List(ctx, metav1.ListOptions{LabelSelector: "owner=helm"})
	if err != nil {
		return HelmReleaseList{Err: err}
	}
	latest := map[string]metav1.PartialObjectMetadata{}
	for _, s := range list.Items {
		name := s.Labels["name"]
		if name == "" {
			continue
		}
		if prev, ok := latest[name]; !ok || leadingInt(s.Labels["version"]) > leadingInt(prev.Labels["version"]) {
			latest[name] = s
		}
	}
	out := HelmReleaseList{}
	for name, s := range latest {
		out.Items = append(out.Items, HelmRelease{Name: name, Status: s.Labels["status"]})
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
	return out
}

func toObjectMeta(m metav1.PartialObjectMetadata) ObjectMeta {
	return ObjectMeta{Namespace: m.Namespace, Name: m.Name, Labels: m.Labels, Annotations: m.Annotations}
}

func collectCRDs(ctx context.Context, meta metadata.Interface, names ...string) CRDList {
	out := CRDList{Items: map[string]ObjectMeta{}}
	for _, name := range names {
		crd, err := meta.Resource(crdsResource).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return CRDList{Err: err}
		}
		out.Items[name] = toObjectMeta(*crd)
	}
	return out
}

func collectDeployments(ctx context.Context, meta metadata.Interface) DeploymentList {
	list, err := meta.Resource(deploymentsResource).List(ctx, metav1.ListOptions{})
	if err != nil {
		return DeploymentList{Err: err}
	}
	out := DeploymentList{}
	for _, d := range list.Items {
		out.Items = append(out.Items, toObjectMeta(d))
	}
	return out
}

func collectStorageClasses(ctx context.Context, c kubernetes.Interface) StorageClassList {
	list, err := c.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return StorageClassList{Err: err}
	}
	out := StorageClassList{}
	for _, sc := range list.Items {
		isDefault := sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" ||
			sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true"
		out.Items = append(out.Items, StorageClass{Name: sc.Name, Default: isDefault, Provisioner: sc.Provisioner})
	}
	return out
}

func collectCSIDrivers(ctx context.Context, c kubernetes.Interface) NameList {
	list, err := c.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err != nil {
		return NameList{Err: err}
	}
	out := NameList{}
	for _, d := range list.Items {
		out.Items = append(out.Items, d.Name)
	}
	return out
}

func collectClusterDNS(ctx context.Context, c kubernetes.Interface) ClusterDNSFacts {
	_, err := c.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return ClusterDNSFacts{}
	}
	if err != nil {
		return ClusterDNSFacts{Err: err}
	}
	slices, err := c.DiscoveryV1().EndpointSlices("kube-system").List(ctx, metav1.ListOptions{LabelSelector: "kubernetes.io/service-name=kube-dns"})
	if err != nil {
		return ClusterDNSFacts{ServiceFound: true, Err: err}
	}
	ready := 0
	for _, s := range slices.Items {
		for _, e := range s.Endpoints {
			// A nil Ready condition means ready (discovery.k8s.io/v1 API contract).
			if e.Conditions.Ready == nil || *e.Conditions.Ready {
				ready++
			}
		}
	}
	return ClusterDNSFacts{ServiceFound: true, ReadyEndpoints: ready}
}

func collectLoadBalancers(ctx context.Context, c kubernetes.Interface) LoadBalancerList {
	list, err := c.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return LoadBalancerList{Err: err}
	}
	out := LoadBalancerList{}
	for _, s := range list.Items {
		if s.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		hasAddress := false
		for _, ing := range s.Status.LoadBalancer.Ingress {
			if ing.IP != "" || ing.Hostname != "" {
				hasAddress = true
			}
		}
		out.Items = append(out.Items, LoadBalancer{Namespace: s.Namespace, Name: s.Name, HasAddress: hasAddress})
	}
	return out
}

func collectMetricsAPI(c kubernetes.Interface) MetricsAPIFacts {
	_, err := c.Discovery().ServerResourcesForGroupVersion("metrics.k8s.io/v1beta1")
	if err == nil {
		return MetricsAPIFacts{Available: true}
	}
	if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "could not find the requested resource") {
		return MetricsAPIFacts{}
	}
	return MetricsAPIFacts{Err: err}
}

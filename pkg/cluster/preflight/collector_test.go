package preflight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

func partial(apiVersion, kind, namespace, name string, labels, annotations map[string]string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: apiVersion, Kind: kind},
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels, Annotations: annotations},
	}
}

func metadataScheme() *runtime.Scheme {
	scheme := metadatafake.NewTestScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

func TestCollectReadsTheCluster(t *testing.T) {
	ready := true
	notReady := false
	clientset := fake.NewClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{corev1.LabelOSStable: "linux"}},
			Spec:       corev1.NodeSpec{ProviderID: "gce://p/europe-west1-b/node-1"},
			Status: corev1.NodeStatus{
				NodeInfo:    corev1.NodeSystemInfo{Architecture: "arm64"},
				Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("7910m"), corev1.ResourceMemory: resource.MustParse("29Gi")},
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "qovery"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
		},
		&storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: "standard-rwo", Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}},
			Provisioner: "pd.csi.storage.gke.io",
		},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kube-dns"}},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kube-dns-abc", Labels: map[string]string{"kubernetes.io/service-name": "kube-dns"}},
			Endpoints: []discoveryv1.Endpoint{
				{Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
				{Conditions: discoveryv1.EndpointConditions{Ready: &notReady}},
				{Conditions: discoveryv1.EndpointConditions{}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			Status:     corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "34.1.2.3"}}}},
		},
	)
	clientset.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{Major: "1", Minor: "33+", GitVersion: "v1.33.5-gke.100"}
	clientset.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		attrs := review.Spec.ResourceAttributes
		review.Status.Allowed = attrs != nil && attrs.Verb == "*" && attrs.Group == "*" && attrs.Resource == "*"
		return true, review, nil
	})

	meta := metadatafake.NewSimpleMetadataClient(metadataScheme(),
		partial("v1", "Secret", "qovery", "sh.helm.release.v1.qovery.v1", map[string]string{"owner": "helm", "name": "qovery", "status": "superseded", "version": "1"}, nil),
		partial("v1", "Secret", "qovery", "sh.helm.release.v1.qovery.v2", map[string]string{"owner": "helm", "name": "qovery", "status": "deployed", "version": "2"}, nil),
		partial("v1", "Secret", "qovery", "unrelated", map[string]string{"app": "x"}, nil),
		partial("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crdGatewayAPI, nil, map[string]string{"gateway.networking.k8s.io/channel": "standard"}),
		partial("apps/v1", "Deployment", "cert-manager", "cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager"}, nil),
	)

	f := collectWith(context.Background(), clientset, meta)

	assert.NoError(t, f.Version.Err)
	assert.Equal(t, 1, f.Version.Major)
	assert.Equal(t, 33, f.Version.Minor)
	assert.True(t, f.Access.ClusterAdmin)

	assert.Len(t, f.Nodes.Items, 1)
	node := f.Nodes.Items[0]
	assert.Equal(t, "linux", node.OS)
	assert.Equal(t, "arm64", node.Arch)
	assert.True(t, node.Ready)
	assert.Equal(t, int64(7910), node.CPUMilli)

	assert.True(t, f.QoveryNS.Exists)
	assert.True(t, f.QoveryNS.Terminating)

	assert.NoError(t, f.HelmReleases.Err)
	assert.Equal(t, []HelmRelease{{Name: "qovery", Status: "deployed"}}, f.HelmReleases.Items)

	assert.NoError(t, f.CRDs.Err)
	assert.Contains(t, f.CRDs.Items, crdGatewayAPI)
	assert.NotContains(t, f.CRDs.Items, crdCertManager)
	assert.Len(t, f.Deployments.Items, 1)

	assert.Equal(t, []StorageClass{{Name: "standard-rwo", Default: true, Provisioner: "pd.csi.storage.gke.io"}}, f.StorageClasses.Items)
	assert.Equal(t, ClusterDNSFacts{ServiceFound: true, ReadyEndpoints: 2}, f.ClusterDNS)
	assert.Equal(t, []LoadBalancer{{Namespace: "default", Name: "web", HasAddress: true}}, f.LoadBalancers.Items)
	assert.False(t, f.MetricsAPI.Available)
	assert.NoError(t, f.MetricsAPI.Err)

	// The collector never writes: the only non-read request is the access review.
	for _, action := range clientset.Actions() {
		verb := action.GetVerb()
		if verb == "create" {
			assert.Equal(t, "selfsubjectaccessreviews", action.GetResource().Resource)
			continue
		}
		assert.Contains(t, []string{"get", "list"}, verb, "unexpected %s %s", verb, action.GetResource().Resource)
	}
	for _, action := range meta.Actions() {
		assert.Contains(t, []string{"get", "list"}, action.GetVerb())
	}

	results := Evaluate(f, Options{})
	assert.True(t, Failed(results))
	assert.Equal(t, StatusFail, resultByID(t, results, "NAMESPACE_TERMINATING").Status)
	assert.Equal(t, StatusFail, resultByID(t, results, "LEGACY_QOVERY_RELEASE").Status)
	assert.Equal(t, StatusFail, resultByID(t, results, "INCOMPATIBLE_CERT_MANAGER").Status)
	assert.Equal(t, StatusWarn, resultByID(t, results, "CRD_OWNERSHIP_CONFLICT").Status)
	assert.Equal(t, StatusPass, resultByID(t, results, "CLOUD_PROVIDER").Status)
	assert.Equal(t, StatusSkip, resultByID(t, results, "EBS_CSI_DRIVER_MISSING").Status)
}

func TestCollectStopsWhenTheAPIIsUnreachable(t *testing.T) {
	clientset := fake.NewClientset()
	clientset.Discovery().(*fakediscovery.FakeDiscovery).PrependReactor("get", "version", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	f := collectWith(context.Background(), clientset, metadatafake.NewSimpleMetadataClient(metadataScheme()))

	assert.Error(t, f.Version.Err)
	// Only the version request was sent; nothing else is tried against an unreachable API.
	assert.Len(t, clientset.Actions(), 1)
	assert.Equal(t, "version", clientset.Actions()[0].GetResource().Resource)
	assert.Equal(t, StatusFail, checkKubernetesVersion(f.Version).Status)
}

package preflight

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// cleanEKS returns the facts of an EKS cluster that is ready for Qovery.
func cleanEKS() Facts {
	return Facts{
		Version: VersionFacts{Raw: "v1.33.2-eks-1", Major: 1, Minor: 33},
		Access:  AccessFacts{ClusterAdmin: true},
		Nodes: NodeList{Items: []Node{{
			Name: "ip-10-0-1-1", Ready: true, OS: "linux", Arch: "amd64",
			ProviderID: "aws:///eu-west-3a/i-0123", CPUMilli: 7910, MemoryBytes: 30 << 30,
			Labels: map[string]string{"eks.amazonaws.com/nodegroup": "default"},
		}}},
		CRDs:           CRDList{Items: map[string]ObjectMeta{}},
		StorageClasses: StorageClassList{Items: []StorageClass{{Name: "gp3", Default: true, Provisioner: ebsCSIDriverName}}},
		CSIDrivers:     NameList{Items: []string{ebsCSIDriverName, "efs.csi.aws.com"}},
		ClusterDNS:     ClusterDNSFacts{ServiceFound: true, ReadyEndpoints: 2},
		LoadBalancers:  LoadBalancerList{Items: []LoadBalancer{{Namespace: "default", Name: "web", HasAddress: true}}},
		MetricsAPI:     MetricsAPIFacts{Available: true},
	}
}

func resultByID(t *testing.T, results []Result, id string) Result {
	t.Helper()
	for _, r := range results {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no result with id %s", id)
	return Result{}
}

func TestEvaluateCleanClusterPasses(t *testing.T) {
	results := Evaluate(cleanEKS(), Options{})

	assert.Len(t, results, len(CheckIDs))
	for i, r := range results {
		assert.Equal(t, CheckIDs[i], r.ID, "checks run in the documented order")
		assert.Equal(t, StatusPass, r.Status, "%s: %s", r.ID, r.Detail)
	}
	assert.False(t, Failed(results))
}

func TestEvaluateUnreachableAPISkipsEverythingElse(t *testing.T) {
	results := Evaluate(Facts{Version: VersionFacts{Err: errors.New("connection refused")}}, Options{})

	assert.Len(t, results, len(CheckIDs))
	assert.Equal(t, StatusFail, results[0].Status)
	for i, r := range results[1:] {
		assert.Equal(t, CheckIDs[i+1], r.ID)
		assert.Equal(t, StatusSkip, r.Status, r.ID)
		assert.NotEmpty(t, r.Title, r.ID)
	}
}

func TestCheckTitlesMatchResults(t *testing.T) {
	for _, r := range Evaluate(cleanEKS(), Options{}) {
		assert.Equal(t, checkTitles[r.ID], r.Title, r.ID)
	}
}

func TestKubernetesVersion(t *testing.T) {
	cases := []struct {
		name   string
		facts  VersionFacts
		status Status
	}{
		{"unreachable API", VersionFacts{Err: errors.New("dial tcp: i/o timeout")}, StatusFail},
		{"1.29 is too old", VersionFacts{Raw: "v1.29.10", Major: 1, Minor: 29}, StatusFail},
		{"0.x is too old", VersionFacts{Raw: "v0.9.0", Major: 0, Minor: 40}, StatusFail},
		{"1.30 warns", VersionFacts{Raw: "v1.30.4", Major: 1, Minor: 30}, StatusWarn},
		{"1.31 passes", VersionFacts{Raw: "v1.31.4+k3s1", Major: 1, Minor: 31}, StatusPass},
		{"1.36 passes", VersionFacts{Raw: "v1.36.0", Major: 1, Minor: 36}, StatusPass},
		{"1.37 is not validated", VersionFacts{Raw: "v1.37.0", Major: 1, Minor: 37}, StatusWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := checkKubernetesVersion(tc.facts)
			assert.Equal(t, tc.status, r.Status, r.Detail)
			if tc.facts.Err == nil && tc.facts.Minor < 31 {
				assert.Contains(t, r.Detail, "1.31 or later")
			}
			if tc.status != StatusPass {
				assert.NotEmpty(t, r.Fix)
			}
		})
	}
}

func TestClusterAdmin(t *testing.T) {
	assert.Equal(t, StatusFail, checkClusterAdmin(AccessFacts{ClusterAdmin: false}).Status)
	assert.Equal(t, StatusSkip, checkClusterAdmin(AccessFacts{Err: errors.New("forbidden")}).Status)
	assert.Equal(t, StatusPass, checkClusterAdmin(AccessFacts{ClusterAdmin: true}).Status)
}

func TestCloudProviderNeverFails(t *testing.T) {
	cases := []struct {
		name       string
		providerID string
		labels     map[string]string
		path       InstallPath
		mention    string
	}{
		{"EKS", "aws:///eu-west-3a/i-1", nil, InstallPathOperator, "AWS"},
		{"GKE", "gce://project/europe-west1-b/gke-node", nil, InstallPathOperator, "GCP"},
		{"Kapsule", "scaleway://instance/fr-par-1/abc", nil, InstallPathOperator, "Scaleway"},
		{"Kapsule by label", "", map[string]string{"k8s.scaleway.com/pool-name": "default"}, InstallPathOperator, "Scaleway"},
		{"AKS", "azure:///subscriptions/x/resourceGroups/y", nil, InstallPathCLI, "Azure"},
		{"k3s", "k3s://node-1", nil, InstallPathCLI, "k3s"},
		{"on-premise", "", nil, InstallPathCLI, "on-premise"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := NodeList{Items: []Node{{Name: "n1", ProviderID: tc.providerID, Labels: tc.labels}}}
			f := Facts{Nodes: nodes}
			path := ResolveInstallPath(f, InstallPathAuto)
			assert.Equal(t, tc.path, path)

			r := checkCloudProvider(nodes, detectProvider(nodes), InstallPathAuto, path)
			assert.Equal(t, StatusPass, r.Status)
			assert.Contains(t, r.Detail, tc.mention)
			if tc.path == InstallPathCLI {
				assert.Contains(t, r.Detail, "qovery cluster install")
				assert.Contains(t, r.Detail, byokLegacyDocURL)
			} else {
				assert.Contains(t, r.Detail, "Qovery Operator installation")
			}
		})
	}
}

func TestInstallPathOverride(t *testing.T) {
	aks := NodeList{Items: []Node{{Name: "n1", ProviderID: "azure:///subscriptions/x"}}}
	r := checkCloudProvider(aks, detectProvider(aks), InstallPathOperator, ResolveInstallPath(Facts{Nodes: aks}, InstallPathOperator))
	assert.Equal(t, StatusWarn, r.Status, "forcing the Operator on an unsupported provider warns, never fails")
	assert.Contains(t, r.Fix, "--install-path cli")

	eks := cleanEKS()
	assert.Equal(t, InstallPathCLI, ResolveInstallPath(eks, InstallPathCLI))
	r = checkCloudProvider(eks.Nodes, detectProvider(eks.Nodes), InstallPathCLI, InstallPathCLI)
	assert.Equal(t, StatusPass, r.Status)
	assert.Contains(t, r.Detail, "also available")

	unreadable := Facts{Nodes: NodeList{Err: errors.New("forbidden")}}
	assert.Equal(t, InstallPathOperator, ResolveInstallPath(unreadable, InstallPathAuto))

	for _, in := range []string{"", "auto", "Operator", "cli"} {
		_, err := ParseInstallPath(in)
		assert.NoError(t, err, in)
	}
	_, err := ParseInstallPath("helm")
	assert.Error(t, err)
}

// onPremise returns the facts of an on-premise cluster (k3s) that already runs cert-manager,
// ExternalDNS, Envoy Gateway and the Qovery release installed by qovery cluster install.
func onPremise() Facts {
	f := cleanEKS()
	f.Nodes = NodeList{Items: []Node{{
		Name: "k3s-1", Ready: true, OS: "linux", Arch: "amd64",
		ProviderID: "k3s://k3s-1", CPUMilli: 8000, MemoryBytes: 32 << 30,
	}}}
	f.HelmReleases = HelmReleaseList{Items: []HelmRelease{{Name: "qovery", Status: "deployed"}}}
	f.CRDs.Items[crdCertManager] = ObjectMeta{Name: crdCertManager, Annotations: map[string]string{
		helmReleaseNameAnnotation: "cert-manager", helmReleaseNamespaceAnnotation: "cert-manager",
	}}
	f.CRDs.Items[crdGatewayAPI] = ObjectMeta{Name: crdGatewayAPI}
	f.Deployments.Items = []ObjectMeta{
		{Namespace: "kube-system", Name: "external-dns", Labels: map[string]string{"app.kubernetes.io/name": "external-dns"}},
		{Namespace: "envoy-gateway-system", Name: "envoy-gateway", Labels: map[string]string{"app.kubernetes.io/name": "gateway-helm"}},
	}
	f.CSIDrivers = NameList{Items: []string{"driver.longhorn.io"}}
	return f
}

func TestCLIInstallPathSkipsOperatorOnlyChecks(t *testing.T) {
	results := Evaluate(onPremise(), Options{})
	assert.False(t, Failed(results), "an on-premise cluster on the CLI installation must not fail on Operator-only checks")

	assert.Equal(t, StatusPass, resultByID(t, results, "CLOUD_PROVIDER").Status)
	legacy := resultByID(t, results, "LEGACY_QOVERY_RELEASE")
	assert.Equal(t, StatusPass, legacy.Status)
	assert.Contains(t, legacy.Detail, "qovery cluster install")
	for _, id := range []string{"INCOMPATIBLE_CERT_MANAGER", "EXISTING_EXTERNAL_DNS", "EXISTING_ENVOY_GATEWAY", "CRD_OWNERSHIP_CONFLICT"} {
		r := resultByID(t, results, id)
		assert.Equal(t, StatusSkip, r.Status, id)
		assert.Contains(t, r.Detail, "Only applies to the Qovery Operator installation", id)
	}

	// The same cluster checked for the Operator installation fails on them.
	forced := Evaluate(onPremise(), Options{InstallPath: InstallPathOperator})
	assert.True(t, Failed(forced))
	assert.Equal(t, StatusWarn, resultByID(t, forced, "CLOUD_PROVIDER").Status)
	assert.Equal(t, StatusFail, resultByID(t, forced, "LEGACY_QOVERY_RELEASE").Status)
	assert.Equal(t, StatusFail, resultByID(t, forced, "INCOMPATIBLE_CERT_MANAGER").Status)
}

func TestNodeArchitectureAndCapacity(t *testing.T) {
	onlyS390x := NodeList{Items: []Node{{Name: "z1", Ready: true, OS: "linux", Arch: "s390x", CPUMilli: 16000, MemoryBytes: 64 << 30}}}
	assert.Equal(t, StatusFail, checkNodeArchitecture(onlyS390x).Status)

	mixed := NodeList{Items: []Node{
		{Name: "a1", Ready: true, OS: "linux", Arch: "arm64"},
		{Name: "w1", Ready: true, OS: "windows", Arch: "amd64"},
	}}
	r := checkNodeArchitecture(mixed)
	assert.Equal(t, StatusWarn, r.Status)
	assert.Contains(t, r.Detail, "w1 (windows/amd64)")

	small := NodeList{Items: []Node{{Name: "s1", Ready: true, OS: "linux", Arch: "amd64", CPUMilli: 3920, MemoryBytes: 15 << 30}}}
	assert.Equal(t, StatusWarn, checkNodeCapacity(small).Status)

	cordoned := NodeList{Items: []Node{{Name: "b1", Ready: true, Unschedulable: true, OS: "linux", Arch: "amd64", CPUMilli: 8000, MemoryBytes: 32 << 30}}}
	assert.Equal(t, StatusWarn, checkNodeCapacity(cordoned).Status)
}

func TestQoveryNamespaceAndLegacyRelease(t *testing.T) {
	assert.Equal(t, StatusFail, checkQoveryNamespace(NamespaceFacts{Exists: true, Terminating: true}).Status)
	assert.Equal(t, StatusPass, checkQoveryNamespace(NamespaceFacts{}).Status)

	legacyRelease := HelmReleaseList{Items: []HelmRelease{{Name: "qovery", Status: "deployed"}}}
	legacy := checkLegacyQoveryRelease(legacyRelease, InstallPathOperator)
	assert.Equal(t, StatusFail, legacy.Status)
	assert.Contains(t, legacy.Fix, "helm -n qovery uninstall qovery")
	assert.Contains(t, legacy.Fix, "--install-path cli")

	assert.Equal(t, StatusPass, checkLegacyQoveryRelease(legacyRelease, InstallPathCLI).Status)

	v2 := checkLegacyQoveryRelease(HelmReleaseList{Items: []HelmRelease{{Name: "qovery-operator", Status: "deployed"}, {Name: "cert-manager", Status: "deployed"}}}, InstallPathOperator)
	assert.Equal(t, StatusPass, v2.Status)
}

func TestCertManager(t *testing.T) {
	t.Run("foreign Helm release fails with its owner as evidence", func(t *testing.T) {
		f := cleanEKS()
		f.CRDs.Items[crdCertManager] = ObjectMeta{Name: crdCertManager, Annotations: map[string]string{
			helmReleaseNameAnnotation: "cert-manager", helmReleaseNamespaceAnnotation: "cert-manager",
		}}
		f.Deployments.Items = []ObjectMeta{{Namespace: "cert-manager", Name: "cert-manager", Labels: map[string]string{"app.kubernetes.io/name": "cert-manager"}}}

		r := checkCertManager(f)
		assert.Equal(t, StatusFail, r.Status)
		assert.Contains(t, r.Detail, "Helm release cert-manager/cert-manager")
		assert.Contains(t, r.Detail, "Deployment cert-manager/cert-manager")
		assert.True(t, Failed(Evaluate(f, Options{})))
	})

	t.Run("CRDs applied with kubectl are foreign on a fresh cluster", func(t *testing.T) {
		f := cleanEKS()
		f.CRDs.Items[crdCertManager] = ObjectMeta{Name: crdCertManager}
		r := checkCertManager(f)
		assert.Equal(t, StatusFail, r.Status)
		assert.Contains(t, r.Detail, "not managed by Helm")
	})

	t.Run("Qovery's own cert-manager passes on a second run", func(t *testing.T) {
		f := cleanEKS()
		f.HelmReleases = HelmReleaseList{Items: []HelmRelease{{Name: "cert-manager", Status: "deployed"}}}
		f.CRDs.Items[crdCertManager] = ObjectMeta{Name: crdCertManager}
		f.Deployments.Items = []ObjectMeta{{
			Namespace: "qovery", Name: "cert-manager",
			Labels:      map[string]string{"app.kubernetes.io/name": "cert-manager"},
			Annotations: map[string]string{helmReleaseNameAnnotation: "cert-manager", helmReleaseNamespaceAnnotation: "qovery"},
		}}
		assert.Equal(t, StatusPass, checkCertManager(f).Status)
	})

	t.Run("unreadable cluster is skipped, never passed", func(t *testing.T) {
		f := cleanEKS()
		f.CRDs = CRDList{Err: errors.New("forbidden")}
		f.Deployments = DeploymentList{Err: errors.New("forbidden")}
		assert.Equal(t, StatusSkip, checkCertManager(f).Status)
	})
}

func TestExternalDNSAndEnvoyGateway(t *testing.T) {
	f := cleanEKS()
	f.Deployments.Items = []ObjectMeta{
		{Namespace: "kube-system", Name: "external-dns", Labels: map[string]string{"app.kubernetes.io/name": "external-dns"}},
		{Namespace: "envoy-gateway-system", Name: "envoy-gateway", Labels: map[string]string{"app.kubernetes.io/name": "gateway-helm"}},
		{Namespace: "qovery", Name: "external-dns", Annotations: map[string]string{helmReleaseNameAnnotation: "external-dns", helmReleaseNamespaceAnnotation: "qovery"}},
	}

	dns := checkExternalDNS(f.Deployments)
	assert.Equal(t, StatusFail, dns.Status)
	assert.Contains(t, dns.Detail, "kube-system/external-dns")
	assert.NotContains(t, dns.Detail, "qovery/external-dns")

	envoy := checkEnvoyGateway(f)
	assert.Equal(t, StatusFail, envoy.Status)
	assert.Contains(t, envoy.Detail, "envoy-gateway-system/envoy-gateway")
}

func TestGatewayAPICRDs(t *testing.T) {
	t.Run("standard channel CRDs from another tool warn", func(t *testing.T) {
		f := cleanEKS()
		f.CRDs.Items[crdGatewayAPI] = ObjectMeta{Name: crdGatewayAPI, Annotations: map[string]string{
			"gateway.networking.k8s.io/channel":        "standard",
			"gateway.networking.k8s.io/bundle-version": "v1.2.1",
		}}
		r := checkGatewayAPICRDs(f)
		assert.Equal(t, StatusWarn, r.Status)
		assert.Contains(t, r.Detail, `channel "standard"`)
		assert.Contains(t, r.Detail, "older than v1.5")
	})

	t.Run("Qovery's own CRDs pass", func(t *testing.T) {
		f := cleanEKS()
		f.CRDs.Items[crdGatewayAPI] = ObjectMeta{Name: crdGatewayAPI, Annotations: map[string]string{
			helmReleaseNameAnnotation: "envoy-gateway-crd", helmReleaseNamespaceAnnotation: "qovery",
		}}
		assert.Equal(t, StatusPass, checkGatewayAPICRDs(f).Status)
	})
}

func TestStorage(t *testing.T) {
	none := StorageClassList{Items: []StorageClass{{Name: "gp2"}}}
	r := checkDefaultStorageClass(none, Options{})
	assert.Equal(t, StatusFail, r.Status)
	assert.Contains(t, r.Detail, "gp2")
	assert.Contains(t, r.Fix, "--loki-object-storage")

	assert.Equal(t, StatusSkip, checkDefaultStorageClass(none, Options{LokiObjectStorage: true}).Status)

	two := StorageClassList{Items: []StorageClass{{Name: "a", Default: true}, {Name: "b", Default: true}}}
	assert.Equal(t, StatusWarn, checkDefaultStorageClass(two, Options{}).Status)

	noEBS := NameList{Items: []string{"efs.csi.aws.com"}}
	assert.Equal(t, StatusFail, checkEBSCSIDriver(noEBS, providerAWS, Options{}).Status)
	assert.Equal(t, StatusWarn, checkEBSCSIDriver(noEBS, providerAWS, Options{LokiObjectStorage: true}).Status)
	assert.Equal(t, StatusSkip, checkEBSCSIDriver(noEBS, providerGCP, Options{}).Status)
}

func TestNetworkChecks(t *testing.T) {
	broken := checkClusterDNS(ClusterDNSFacts{ServiceFound: true, ReadyEndpoints: 0})
	assert.Equal(t, StatusFail, broken.Status)
	assert.Contains(t, broken.Fix, "nslookup engine.qovery.com")
	assert.Equal(t, StatusWarn, checkClusterDNS(ClusterDNSFacts{}).Status)

	pending := checkLoadBalancer(LoadBalancerList{Items: []LoadBalancer{{Namespace: "default", Name: "web"}}}, providerAWS)
	assert.Equal(t, StatusWarn, pending.Status)
	assert.Contains(t, pending.Fix, "kubernetes.io/role/elb=1")
	assert.Equal(t, StatusSkip, checkLoadBalancer(LoadBalancerList{}, providerGCP).Status)

	assert.Equal(t, StatusWarn, checkMetricsServer(MetricsAPIFacts{}).Status)
}

func TestEvidenceIsBounded(t *testing.T) {
	var items []string
	for i := 0; i < 10; i++ {
		items = append(items, "x")
	}
	joined := joinEvidence(items)
	assert.True(t, strings.HasSuffix(joined, "and 6 more"), joined)
}

package preflight

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Status of one check. Only FAIL makes the preflight fail. SKIP means the check could not be
// evaluated or does not apply; it is never a PASS.
type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

// Result of one check. ID is stable; where a check matches a q-core platform preflight check
// it reuses that id (q-core: platform/preflight/domain/ClusterPlatformPreflight.kt), so the
// CLI and the deployment logs speak the same language.
type Result struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// InstallPath is the way Qovery gets installed on the cluster.
//
// Qovery runs on any Kubernetes cluster, with one of two installations:
//   - InstallPathOperator: the Qovery Operator Helm chart shown in the Qovery Console. It
//     supports EKS (AWS), GKE (GCP) and Scaleway Kapsule.
//   - InstallPathCLI: the installation with the Qovery CLI (qovery cluster install), for every
//     other cluster (AKS, k3s, RKE2, OpenShift, on-premise, ...).
//
// Some checks only apply to the Operator installation (it installs its own cert-manager,
// ExternalDNS, Envoy Gateway and Gateway API CRDs, and does not take over a legacy `qovery`
// release). They are skipped on the CLI installation.
type InstallPath string

const (
	// InstallPathAuto picks the installation from the provider of the nodes.
	InstallPathAuto     InstallPath = ""
	InstallPathOperator InstallPath = "operator"
	InstallPathCLI      InstallPath = "cli"
)

// ParseInstallPath reads the value of the --install-path flag.
func ParseInstallPath(s string) (InstallPath, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return InstallPathAuto, nil
	case "operator":
		return InstallPathOperator, nil
	case "cli":
		return InstallPathCLI, nil
	}
	return InstallPathAuto, fmt.Errorf("invalid install path %q: use auto, operator or cli", s)
}

// Options change which checks apply.
type Options struct {
	// LokiObjectStorage is true when Loki will store logs in object storage (s3, gcs,
	// s3Compatible) instead of the default pvc storage. Loki then needs no volume.
	LokiObjectStorage bool
	// InstallPath forces the installation to check for. InstallPathAuto (the default) picks
	// it from the provider of the nodes, see ResolveInstallPath.
	InstallPath InstallPath
}

const byokLegacyDocURL = "https://www.qovery.com/docs/configuration/integrations/kubernetes/byok-legacy"

const (
	qoveryNamespace = "qovery"

	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"

	crdCertManager   = "certificates.cert-manager.io"
	crdEnvoyProxy    = "envoyproxies.gateway.envoyproxy.io"
	crdGatewayAPI    = "gateways.gateway.networking.k8s.io"
	ebsCSIDriverName = "ebs.csi.aws.com"

	// Minimum CPU and memory a single node must offer: each build runs in a builder pod that
	// requests 4000m and 8 GiB by default (documentation: byok.mdx, Prerequisites).
	builderCPUMilli    = int64(4000)
	builderMemoryBytes = int64(8) << 30
)

// CheckIDs lists every check in execution order.
var CheckIDs = []string{
	"KUBERNETES_VERSION",
	"RBAC_INSUFFICIENT",
	"CLOUD_PROVIDER",
	"NODE_ARCHITECTURE_UNSUPPORTED",
	"NODE_CAPACITY_INSUFFICIENT",
	"NAMESPACE_TERMINATING",
	"LEGACY_QOVERY_RELEASE",
	"INCOMPATIBLE_CERT_MANAGER",
	"EXISTING_EXTERNAL_DNS",
	"EXISTING_ENVOY_GATEWAY",
	"CRD_OWNERSHIP_CONFLICT",
	"DEFAULT_STORAGE_CLASS_MISSING",
	"EBS_CSI_DRIVER_MISSING",
	"CLUSTER_DNS_UNHEALTHY",
	"LOAD_BALANCER_UNVERIFIED",
	"METRICS_SERVER_MISSING",
}

var checkTitles = map[string]string{
	"KUBERNETES_VERSION":            "Kubernetes API and version",
	"RBAC_INSUFFICIENT":             "cluster-admin permissions",
	"CLOUD_PROVIDER":                "Cloud provider and installation",
	"NODE_ARCHITECTURE_UNSUPPORTED": "Node OS and architecture",
	"NODE_CAPACITY_INSUFFICIENT":    "Node size for builds",
	"NAMESPACE_TERMINATING":         "Namespace qovery",
	"LEGACY_QOVERY_RELEASE":         "Legacy Qovery installation",
	"INCOMPATIBLE_CERT_MANAGER":     "Existing cert-manager",
	"EXISTING_EXTERNAL_DNS":         "Existing ExternalDNS",
	"EXISTING_ENVOY_GATEWAY":        "Existing Envoy Gateway",
	"CRD_OWNERSHIP_CONFLICT":        "Existing Gateway API CRDs",
	"DEFAULT_STORAGE_CLASS_MISSING": "Default StorageClass",
	"EBS_CSI_DRIVER_MISSING":        "Amazon EBS CSI driver",
	"CLUSTER_DNS_UNHEALTHY":         "Cluster DNS",
	"LOAD_BALANCER_UNVERIFIED":      "LoadBalancer Services",
	"METRICS_SERVER_MISSING":        "metrics-server",
}

// Evaluate runs every check over the collected facts. It never touches the cluster.
func Evaluate(f Facts, opts Options) []Result {
	if f.Version.Err != nil {
		// Nothing else was read: report every other check as not evaluated rather than
		// evaluating empty facts.
		results := []Result{checkKubernetesVersion(f.Version)}
		for _, id := range CheckIDs[1:] {
			results = append(results, Result{ID: id, Title: checkTitles[id], Status: StatusSkip, Detail: "Not evaluated: the Kubernetes API is unreachable."})
		}
		return results
	}
	provider := detectProvider(f.Nodes)
	path := ResolveInstallPath(f, opts.InstallPath)
	operatorOnly := func(check Result) Result {
		if path == InstallPathOperator {
			return check
		}
		return Result{
			ID:     check.ID,
			Title:  check.Title,
			Status: StatusSkip,
			Detail: "Only applies to the Qovery Operator installation. qovery cluster install reuses the components already on the cluster.",
		}
	}
	return []Result{
		checkKubernetesVersion(f.Version),
		checkClusterAdmin(f.Access),
		checkCloudProvider(f.Nodes, provider, opts.InstallPath, path),
		checkNodeArchitecture(f.Nodes),
		checkNodeCapacity(f.Nodes),
		checkQoveryNamespace(f.QoveryNS),
		checkLegacyQoveryRelease(f.HelmReleases, path),
		operatorOnly(checkCertManager(f)),
		operatorOnly(checkExternalDNS(f.Deployments)),
		operatorOnly(checkEnvoyGateway(f)),
		operatorOnly(checkGatewayAPICRDs(f)),
		checkDefaultStorageClass(f.StorageClasses, opts),
		checkEBSCSIDriver(f.CSIDrivers, provider, opts),
		checkClusterDNS(f.ClusterDNS),
		checkLoadBalancer(f.LoadBalancers, provider),
		checkMetricsServer(f.MetricsAPI),
	}
}

// ResolveInstallPath returns the installation the checks are run for. A forced path wins.
// Otherwise the Operator installation is picked for AWS, GCP and Scaleway nodes, and the CLI
// installation for every other provider. When the nodes cannot be read, the Operator
// installation is assumed, because its checks are the stricter ones.
func ResolveInstallPath(f Facts, requested InstallPath) InstallPath {
	if requested != InstallPathAuto {
		return requested
	}
	if f.Nodes.Err != nil {
		return InstallPathOperator
	}
	if operatorProvider(detectProvider(f.Nodes)) {
		return InstallPathOperator
	}
	return InstallPathCLI
}

// Failed reports whether at least one check failed.
func Failed(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusFail {
			return true
		}
	}
	return false
}

// Kubernetes version.
//
// Source: the Gateway API layer of the self-managed template installs the gateway-crds-helm
// chart v1.9.1 with crds.gatewayAPI.channel=experimental (q-core fixture
// platform-catalog/components/envoy-gateway-crd/config/static-values/base.yaml, mirrored from
// the Engine catalog). That chart renders a ValidatingAdmissionPolicy in
// admissionregistration.k8s.io/v1, an API that only exists from Kubernetes 1.30, and its
// TLSRoute CRD uses the CEL isIP() function, which the API server accepts from Kubernetes 1.31.
// The upper bound is the newest version q-core models (organization/model/KubernetesVersion.kt).
const (
	minKubernetesMinor         = 30
	recommendedKubernetesMinor = 31
	newestKnownKubernetesMinor = 36
)

func checkKubernetesVersion(v VersionFacts) Result {
	r := Result{ID: "KUBERNETES_VERSION", Title: "Kubernetes API and version"}
	if v.Err != nil {
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("Cannot reach the Kubernetes API: %v", v.Err)
		r.Fix = "Check that your kubeconfig points to the cluster (kubectl config current-context) and that you can reach its API server (VPN, private endpoint, expired credentials)."
		return r
	}
	switch {
	case v.Major != 1 || v.Minor < minKubernetesMinor:
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("Kubernetes %s is too old. Qovery supports Kubernetes 1.%d or later.", v.Raw, recommendedKubernetesMinor)
		r.Fix = fmt.Sprintf("Upgrade the cluster to Kubernetes 1.%d or later.", recommendedKubernetesMinor)
	case v.Minor < recommendedKubernetesMinor:
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("Kubernetes %s is older than the supported versions (1.%d or later). Some components need 1.31, for example the Gateway API CRDs of the Qovery Operator installation (their TLSRoute validation uses a CEL function added in Kubernetes 1.31).", v.Raw, recommendedKubernetesMinor)
		r.Fix = fmt.Sprintf("Upgrade the cluster to Kubernetes 1.%d or later.", recommendedKubernetesMinor)
	case v.Minor > newestKnownKubernetesMinor:
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("Kubernetes %s is newer than the versions Qovery has validated (up to 1.%d).", v.Raw, newestKnownKubernetesMinor)
		r.Fix = "The installation can still work. If it fails, contact Qovery support with the deployment logs."
	default:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("Kubernetes %s", v.Raw)
	}
	return r
}

// Permissions.
//
// Source: the qovery-operator chart binds the worker ServiceAccount to cluster-admin
// (values: workerRbac.clusterAdmin=true), and Kubernetes only lets a user create a binding
// for permissions they already hold. The documentation asks for cluster-admin rights
// (byok.mdx, Prerequisites). Matches q-core RBAC_INSUFFICIENT.
func checkClusterAdmin(a AccessFacts) Result {
	r := Result{ID: "RBAC_INSUFFICIENT", Title: "cluster-admin permissions"}
	switch {
	case a.Err != nil:
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not verify your permissions: %v", a.Err)
		r.Fix = "Run kubectl auth can-i '*' '*' --all-namespaces. It must answer yes."
	case !a.ClusterAdmin:
		r.Status = StatusFail
		r.Detail = "Your kubeconfig user does not have cluster-admin rights."
		r.Fix = "Use a kubeconfig user bound to the cluster-admin ClusterRole. Qovery grants cluster-admin to its agents, and Kubernetes only lets you grant permissions you hold."
	default:
		r.Status = StatusPass
		r.Detail = "Your kubeconfig user has cluster-admin rights."
	}
	return r
}

// Cloud provider and installation.
//
// Source: q-core accepts a self-managed cluster run by the Qovery Operator for AWS, GCP and
// Scaleway only (platform/selfmanagedcluster/domain/SelfManagedCluster.kt). Every other cluster
// is installed with the Qovery CLI (qovery cluster install, documentation byok-legacy.mdx).
// This check never fails: it tells which installation to use.
type provider string

const (
	providerAWS      provider = "AWS"
	providerGCP      provider = "GCP"
	providerScaleway provider = "Scaleway"
	providerAzure    provider = "Azure"
	providerUnknown  provider = ""
)

func nodeProvider(n Node) provider {
	id := strings.ToLower(n.ProviderID)
	switch {
	case strings.HasPrefix(id, "aws://"):
		return providerAWS
	case strings.HasPrefix(id, "gce://"):
		return providerGCP
	case strings.HasPrefix(id, "scaleway://"):
		return providerScaleway
	case strings.HasPrefix(id, "azure://"):
		return providerAzure
	}
	if _, ok := n.Labels["k8s.scaleway.com/pool-name"]; ok {
		return providerScaleway
	}
	return providerUnknown
}

func detectProvider(nodes NodeList) provider {
	counts := map[provider]int{}
	for _, n := range nodes.Items {
		counts[nodeProvider(n)]++
	}
	best, bestCount := providerUnknown, 0
	for p, c := range counts {
		if c > bestCount || (c == bestCount && p > best) {
			best, bestCount = p, c
		}
	}
	return best
}

func providerDistribution(nodes NodeList) string {
	for _, n := range nodes.Items {
		id := strings.ToLower(n.ProviderID)
		switch {
		case strings.HasPrefix(id, "k3s://"):
			return "k3s"
		case strings.HasPrefix(id, "kind://"):
			return "kind"
		case strings.HasPrefix(id, "rke2://"):
			return "RKE2"
		}
		if n.Labels["node.kubernetes.io/instance-type"] == "k3s" {
			return "k3s"
		}
	}
	return ""
}

func operatorProvider(p provider) bool {
	return p == providerAWS || p == providerGCP || p == providerScaleway
}

func providerName(nodes NodeList, p provider) string {
	if p != providerUnknown {
		return string(p)
	}
	if d := providerDistribution(nodes); d != "" {
		return d
	}
	return "an unrecognized provider or on-premise"
}

func checkCloudProvider(nodes NodeList, p provider, requested, path InstallPath) Result {
	r := Result{ID: "CLOUD_PROVIDER", Title: "Cloud provider and installation"}
	if nodes.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read the nodes: %v. Checked for the Qovery Operator installation; run with --install-path cli if you install with qovery cluster install.", nodes.Err)
		return r
	}
	name := providerName(nodes, p)
	supported := operatorProvider(p)
	switch {
	case path == InstallPathOperator && supported:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("%s cluster. Use the Qovery Operator installation (the Helm command shown in the Qovery Console).", name)
	case path == InstallPathOperator:
		// Only reachable with --install-path operator on another provider.
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("The nodes run on %s. The Qovery Operator installation supports EKS, GKE and Scaleway Kapsule only.", name)
		r.Fix = "Install Qovery with the Qovery CLI (qovery cluster install) and run the preflight with --install-path cli: " + byokLegacyDocURL
	case requested == InstallPathCLI && supported:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("%s cluster, checked for the installation with the Qovery CLI (qovery cluster install). The Qovery Operator installation is also available for %s.", name, name)
	default:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("The nodes run on %s. Install Qovery with the Qovery CLI: qovery cluster install (%s). The Qovery Operator installation covers EKS, GKE and Scaleway Kapsule.", name, byokLegacyDocURL)
	}
	return r
}

// Node architecture.
//
// Source: the qovery-operator, cluster-agent and shell-agent images on
// public.ecr.aws/r3m4q3r9 are published for linux/amd64 and linux/arm64 only.
func checkNodeArchitecture(nodes NodeList) Result {
	r := Result{ID: "NODE_ARCHITECTURE_UNSUPPORTED", Title: "Node OS and architecture"}
	if nodes.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read the nodes: %v", nodes.Err)
		return r
	}
	supported := 0
	var others []string
	for _, n := range nodes.Items {
		if n.OS == "linux" && (n.Arch == "amd64" || n.Arch == "arm64") {
			if n.Ready {
				supported++
			}
			continue
		}
		others = append(others, fmt.Sprintf("%s (%s/%s)", n.Name, n.OS, n.Arch))
	}
	switch {
	case supported == 0:
		r.Status = StatusFail
		r.Detail = "No Ready Linux node with an amd64 or arm64 CPU."
		r.Fix = "Add a Linux node pool with amd64 or arm64 nodes. Qovery images are published for these architectures only."
	case len(others) > 0:
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("%d Ready Linux amd64/arm64 node(s). Qovery cannot run on: %s.", supported, strings.Join(others, ", "))
		r.Fix = "Keep at least one Linux amd64 or arm64 node pool for Qovery and its builds."
	default:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("%d Ready Linux amd64/arm64 node(s)", supported)
	}
	return r
}

// Node capacity.
//
// Source: documentation byok.mdx, Prerequisites (builder pod requests 4000m and 8 GiB).
// Allocatable is an upper bound of what is free, so a PASS here does not prove free capacity.
func checkNodeCapacity(nodes NodeList) Result {
	r := Result{ID: "NODE_CAPACITY_INSUFFICIENT", Title: "Node size for builds"}
	if nodes.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read the nodes: %v", nodes.Err)
		return r
	}
	for _, n := range nodes.Items {
		if n.Ready && !n.Unschedulable && n.OS == "linux" && n.CPUMilli >= builderCPUMilli && n.MemoryBytes >= builderMemoryBytes {
			r.Status = StatusPass
			r.Detail = fmt.Sprintf("Node %s can host a builder pod (4 vCPU, 8 GiB) when it has that much free.", n.Name)
			return r
		}
	}
	r.Status = StatusWarn
	r.Detail = "No schedulable node has 4 vCPU and 8 GiB allocatable. Builds with the default settings will stay on Provisioning builder."
	r.Fix = "Add a node with at least 4 vCPU and 8 GiB free (for example an 8 vCPU node), or lower CPU and RAM in the build settings of each service. With a node autoscaler such as Karpenter, make sure it can create such a node."
	return r
}

// Target namespace.
//
// Source: q-core NAMESPACE_TERMINATING; the Operator chart installs into namespace qovery.
func checkQoveryNamespace(ns NamespaceFacts) Result {
	r := Result{ID: "NAMESPACE_TERMINATING", Title: "Namespace qovery"}
	switch {
	case ns.Err != nil:
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read namespace qovery: %v", ns.Err)
	case ns.Terminating:
		r.Status = StatusFail
		r.Detail = "Namespace qovery is being deleted."
		r.Fix = "Wait until kubectl get namespace qovery returns NotFound, then install again. If it stays Terminating, check the finalizers of the resources left in it."
	case !ns.Exists:
		r.Status = StatusPass
		r.Detail = "Namespace qovery does not exist yet. The installation creates it."
	default:
		r.Status = StatusPass
		r.Detail = "Namespace qovery exists and is active."
	}
	return r
}

// Legacy installation.
//
// Source: q-core doc-features/engine-v2/roadmap/demo-operator-engine-v2.md ("No implicit legacy
// Helm adoption"): the legacy umbrella release `qovery` (installed by `qovery cluster install`
// and the legacy demo) and the Engine v2 component releases have different Helm ownership, and
// Engine v2 does not adopt it.
//
// On the CLI installation, release `qovery` is the installation itself, so it is reported
// and never fails.
func checkLegacyQoveryRelease(releases HelmReleaseList, path InstallPath) Result {
	r := Result{ID: "LEGACY_QOVERY_RELEASE", Title: "Legacy Qovery installation"}
	if releases.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not list the Helm releases of namespace qovery: %v", releases.Err)
		return r
	}
	for _, rel := range releases.Items {
		if rel.Name != "qovery" {
			continue
		}
		if path == InstallPathCLI {
			r.Status = StatusPass
			r.Detail = fmt.Sprintf("Namespace qovery holds the Qovery Helm release qovery (status %s), installed with the Qovery CLI. qovery cluster install manages this release.", rel.Status)
			return r
		}
		r.Status = StatusFail
		r.Detail = fmt.Sprintf("Namespace qovery holds the legacy Qovery Helm release qovery (status %s).", rel.Status)
		r.Fix = "The Qovery Operator does not take over an installation made with qovery cluster install. Keep that installation (run the preflight with --install-path cli), or uninstall it (helm -n qovery uninstall qovery) and remove its cert-manager, ExternalDNS and ingress resources before installing the Operator."
		return r
	}
	r.Status = StatusPass
	r.Detail = "No legacy Qovery Helm release found."
	return r
}

// Ownership helpers.

func helmOwner(m ObjectMeta) string {
	name := m.Annotations[helmReleaseNameAnnotation]
	if name == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", m.Annotations[helmReleaseNamespaceAnnotation], name)
}

func ownerDescription(m ObjectMeta) string {
	if owner := helmOwner(m); owner != "" {
		return "Helm release " + owner
	}
	return "not managed by Helm"
}

// ownedByQovery tells whether an object belongs to one of the given Qovery releases in
// namespace qovery. An object without Helm annotations is considered Qovery's when that
// release already exists in namespace qovery: Engine v2 applies CRDs with server-side apply
// (catalog applyStrategy SERVER_SIDE_APPLY_CRDS), which can leave them without Helm metadata.
func ownedByQovery(m ObjectMeta, releases HelmReleaseList, names ...string) bool {
	if m.Annotations[helmReleaseNamespaceAnnotation] == qoveryNamespace {
		for _, n := range names {
			if m.Annotations[helmReleaseNameAnnotation] == n {
				return true
			}
		}
		return false
	}
	if helmOwner(m) != "" {
		return false
	}
	for _, rel := range releases.Items {
		for _, n := range names {
			if rel.Name == n {
				return true
			}
		}
	}
	return false
}

func deploymentMatches(d ObjectMeta, labelNames []string, names []string) bool {
	for _, key := range []string{"app.kubernetes.io/name", "app"} {
		for _, v := range labelNames {
			if d.Labels[key] == v {
				return true
			}
		}
	}
	for _, n := range names {
		if d.Name == n {
			return true
		}
	}
	return false
}

func foreignDeployments(deps DeploymentList, releases HelmReleaseList, labelNames, names, qoveryReleases []string) []string {
	var found []string
	for _, d := range deps.Items {
		if !deploymentMatches(d, labelNames, names) {
			continue
		}
		if d.Namespace == qoveryNamespace && ownedByQovery(d, releases, qoveryReleases...) {
			continue
		}
		found = append(found, fmt.Sprintf("Deployment %s/%s (%s)", d.Namespace, d.Name, ownerDescription(d)))
	}
	sort.Strings(found)
	return found
}

func foreignCRD(crds CRDList, releases HelmReleaseList, name string, qoveryReleases ...string) string {
	m, ok := crds.Items[name]
	if !ok || ownedByQovery(m, releases, qoveryReleases...) {
		return ""
	}
	return fmt.Sprintf("CRD %s (%s)", name, ownerDescription(m))
}

func joinEvidence(items []string) string {
	const max = 4
	if len(items) > max {
		return strings.Join(items[:max], "; ") + fmt.Sprintf("; and %d more", len(items)-max)
	}
	return strings.Join(items, "; ")
}

// cert-manager.
//
// Source: the dns-certificates layer installs cert-manager v1.20.0 as release cert-manager in
// namespace qovery. The documentation requires a cluster without cert-manager and states that
// Qovery does not adopt an existing installation (byok.mdx, Prerequisites and Current
// limitations); doc-features/engine-v2/guides/byok-end-to-end.md says the same. Matches q-core
// INCOMPATIBLE_CERT_MANAGER, which today only warns during the deployment.
func checkCertManager(f Facts) Result {
	r := Result{ID: "INCOMPATIBLE_CERT_MANAGER", Title: "Existing cert-manager"}
	if f.CRDs.Err != nil && f.Deployments.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read CRDs or Deployments: %v", f.CRDs.Err)
		return r
	}
	var evidence []string
	if e := foreignCRD(f.CRDs, f.HelmReleases, crdCertManager, "cert-manager"); e != "" {
		evidence = append(evidence, e)
	}
	evidence = append(evidence, foreignDeployments(f.Deployments, f.HelmReleases,
		[]string{"cert-manager"}, []string{"cert-manager"}, []string{"cert-manager"})...)
	if len(evidence) == 0 {
		r.Status = StatusPass
		r.Detail = "No cert-manager installation found outside Qovery."
		return r
	}
	r.Status = StatusFail
	r.Detail = "cert-manager is already installed: " + joinEvidence(evidence) + "."
	r.Fix = "Qovery installs its own cert-manager in namespace qovery and does not take over an existing one. Use a cluster without cert-manager, or uninstall it and its CRDs if nothing else on the cluster uses it."
	return r
}

// ExternalDNS.
//
// Source: documentation byok.mdx, Prerequisites and Current limitations; the dns-certificates
// layer installs ExternalDNS as release external-dns in namespace qovery.
func checkExternalDNS(deps DeploymentList) Result {
	r := Result{ID: "EXISTING_EXTERNAL_DNS", Title: "Existing ExternalDNS"}
	if deps.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read Deployments: %v", deps.Err)
		return r
	}
	var evidence []string
	for _, d := range deps.Items {
		matches := d.Labels["app.kubernetes.io/name"] == "external-dns" || d.Labels["app"] == "external-dns" || strings.Contains(d.Name, "external-dns")
		if !matches || (d.Namespace == qoveryNamespace && d.Annotations[helmReleaseNamespaceAnnotation] == qoveryNamespace) {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("Deployment %s/%s (%s)", d.Namespace, d.Name, ownerDescription(d)))
	}
	sort.Strings(evidence)
	if len(evidence) == 0 {
		r.Status = StatusPass
		r.Detail = "No ExternalDNS installation found outside Qovery."
		return r
	}
	r.Status = StatusFail
	r.Detail = "ExternalDNS is already installed: " + joinEvidence(evidence) + "."
	r.Fix = "Qovery installs its own ExternalDNS for the Qovery-provided domain and does not take over an existing one. Use a cluster without ExternalDNS, or uninstall it if nothing else uses it."
	return r
}

// Envoy Gateway.
//
// Source: the gateway-api layer installs Envoy Gateway v1.9.1 (releases envoy-gateway-crd and
// envoy-gateway in namespace qovery). The documentation requires a cluster without Envoy
// Gateway (byok.mdx, Prerequisites and Current limitations).
func checkEnvoyGateway(f Facts) Result {
	r := Result{ID: "EXISTING_ENVOY_GATEWAY", Title: "Existing Envoy Gateway"}
	if f.CRDs.Err != nil && f.Deployments.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read CRDs or Deployments: %v", f.CRDs.Err)
		return r
	}
	var evidence []string
	if e := foreignCRD(f.CRDs, f.HelmReleases, crdEnvoyProxy, "envoy-gateway-crd"); e != "" {
		evidence = append(evidence, e)
	}
	evidence = append(evidence, foreignDeployments(f.Deployments, f.HelmReleases,
		[]string{"gateway-helm"}, []string{"envoy-gateway"}, []string{"envoy-gateway"})...)
	if len(evidence) == 0 {
		r.Status = StatusPass
		r.Detail = "No Envoy Gateway installation found outside Qovery."
		return r
	}
	r.Status = StatusFail
	r.Detail = "Envoy Gateway is already installed: " + joinEvidence(evidence) + "."
	r.Fix = "Qovery installs its own Envoy Gateway in namespace qovery and does not take over an existing one. Use a cluster without Envoy Gateway, or uninstall it and its CRDs if nothing else uses it."
	return r
}

var gatewayBundleBefore15 = regexp.MustCompile(`^v1\.[0-4](\.|$)`)

// Gateway API CRDs.
//
// Source: release envoy-gateway-crd installs the Gateway API CRDs from the experimental
// channel, bundle v1.6.1, with server-side apply (q-core fixture base.yaml above; catalog
// applyStrategy SERVER_SIDE_APPLY_CRDS). The same chart installs the ValidatingAdmissionPolicy
// safe-upgrades.gateway.networking.k8s.io, which refuses Gateway API CRDs older than v1.5.
// Matches q-core CRD_OWNERSHIP_CONFLICT, which today only warns during the deployment.
func checkGatewayAPICRDs(f Facts) Result {
	r := Result{ID: "CRD_OWNERSHIP_CONFLICT", Title: "Existing Gateway API CRDs"}
	if f.CRDs.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read CRDs: %v", f.CRDs.Err)
		return r
	}
	m, ok := f.CRDs.Items[crdGatewayAPI]
	if !ok {
		r.Status = StatusPass
		r.Detail = "No Gateway API CRDs installed yet. Qovery installs them."
		return r
	}
	if ownedByQovery(m, f.HelmReleases, "envoy-gateway-crd") {
		r.Status = StatusPass
		r.Detail = "The Gateway API CRDs belong to the Qovery installation."
		return r
	}
	channel := m.Annotations["gateway.networking.k8s.io/channel"]
	bundle := m.Annotations["gateway.networking.k8s.io/bundle-version"]
	r.Status = StatusWarn
	r.Detail = fmt.Sprintf("The Gateway API CRDs are already installed (%s, channel %q, bundle %q). Qovery replaces them with the experimental channel, bundle v1.6.1.", ownerDescription(m), channel, bundle)
	if gatewayBundleBefore15.MatchString(bundle) {
		r.Detail += " Bundles older than v1.5 are refused by the Gateway API upgrade policy Qovery installs."
	}
	r.Fix = "If another tool installed and reconciles these CRDs (GKE Gateway API, Istio, Cilium, a load balancer controller), it can conflict with Qovery. On GKE, disable the GKE-managed Gateway API if nothing uses it. Otherwise, contact Qovery support before installing."
	return r
}

// Default StorageClass.
//
// Source: Loki's default pvc storage needs a default StorageClass (documentation byok.mdx,
// Prerequisites and the "Loki stays Pending" troubleshooting entry). Matches q-core
// DEFAULT_STORAGE_CLASS_MISSING.
func checkDefaultStorageClass(scs StorageClassList, opts Options) Result {
	r := Result{ID: "DEFAULT_STORAGE_CLASS_MISSING", Title: "Default StorageClass"}
	if opts.LokiObjectStorage {
		r.Status = StatusSkip
		r.Detail = "Loki uses object storage and needs no volume."
		return r
	}
	if scs.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read StorageClasses: %v", scs.Err)
		return r
	}
	var defaults, all []string
	for _, sc := range scs.Items {
		all = append(all, sc.Name)
		if sc.Default {
			defaults = append(defaults, sc.Name)
		}
	}
	switch {
	case len(defaults) == 0:
		r.Status = StatusFail
		r.Detail = "No default StorageClass. Loki stores logs on a volume by default and will stay Pending."
		if len(all) > 0 {
			r.Detail += fmt.Sprintf(" StorageClasses found: %s.", strings.Join(all, ", "))
		}
		r.Fix = "Mark one as default: kubectl annotate storageclass <NAME> storageclass.kubernetes.io/is-default-class=true. Or choose object storage for Loki before the first deployment and run the preflight with --loki-object-storage."
	case len(defaults) > 1:
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("Several default StorageClasses: %s. Kubernetes picks the most recent one.", strings.Join(defaults, ", "))
		r.Fix = "Keep a single default StorageClass."
	default:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("Default StorageClass: %s", defaults[0])
	}
	return r
}

// EBS CSI driver.
//
// Source: documentation byok.mdx, Prerequisites: on EKS, the Loki pvc storage, container
// databases and service storage all use the Amazon EBS CSI driver add-on.
func checkEBSCSIDriver(drivers NameList, p provider, opts Options) Result {
	r := Result{ID: "EBS_CSI_DRIVER_MISSING", Title: "Amazon EBS CSI driver"}
	if p != providerAWS {
		r.Status = StatusSkip
		r.Detail = "Only needed on AWS."
		return r
	}
	if drivers.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read CSIDrivers: %v", drivers.Err)
		return r
	}
	for _, d := range drivers.Items {
		if d == ebsCSIDriverName {
			r.Status = StatusPass
			r.Detail = "The Amazon EBS CSI driver is installed."
			return r
		}
	}
	r.Status = StatusWarn
	if !opts.LokiObjectStorage {
		r.Status = StatusFail
	}
	r.Detail = "The Amazon EBS CSI driver is not installed. Volumes for Loki, container databases and service storage cannot be created."
	r.Fix = "Add the Amazon EBS CSI driver add-on to the EKS cluster, with its IAM role."
	return r
}

// Cluster DNS.
//
// Source: doc-features/engine-v2/guides/byok-end-to-end.md, "Validate foundational cluster
// DNS": the Operator resolves the Qovery gateway through the cluster DNS before it connects.
// Matches q-core CLUSTER_DNS_UNHEALTHY. This check reads the Service and its endpoints only;
// it does not start a probe Pod.
func checkClusterDNS(dns ClusterDNSFacts) Result {
	r := Result{ID: "CLUSTER_DNS_UNHEALTHY", Title: "Cluster DNS"}
	const lookup = "kubectl -n default run qovery-dns-preflight --rm -i --restart=Never --image=busybox:1.36 -- nslookup engine.qovery.com"
	switch {
	case dns.Err != nil:
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read the cluster DNS Service: %v", dns.Err)
		r.Fix = "Check name resolution from a Pod: " + lookup
	case !dns.ServiceFound:
		r.Status = StatusWarn
		r.Detail = "No kube-dns Service in namespace kube-system, so the cluster DNS could not be verified."
		r.Fix = "Check name resolution from a Pod. The lookup must return an address: " + lookup
	case dns.ReadyEndpoints == 0:
		r.Status = StatusFail
		r.Detail = "The kube-dns Service has no ready endpoint. The Qovery agents cannot resolve Qovery and will not connect."
		r.Fix = "Fix CoreDNS (kubectl -n kube-system get pods -l k8s-app=kube-dns) before installing, then check: " + lookup
	default:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("kube-dns has %d ready endpoint(s). Resolution of public names is not tested; run: %s", dns.ReadyEndpoints, lookup)
	}
	return r
}

// LoadBalancer support.
//
// Source: documentation byok.mdx and byok-legacy.mdx, Prerequisites: Envoy Gateway (Operator
// installation) or the NGINX Ingress Controller (CLI installation) exposes applications through
// a public Service of type LoadBalancer. Without a probe Service, support can only be inferred
// from existing LoadBalancer Services.
func checkLoadBalancer(lbs LoadBalancerList, p provider) Result {
	r := Result{ID: "LOAD_BALANCER_UNVERIFIED", Title: "LoadBalancer Services"}
	fix := "The cluster must create a public load balancer for a Service of type LoadBalancer (on-premise, use MetalLB or an equivalent)."
	if p == providerAWS {
		fix += " On EKS, install the AWS Load Balancer Controller or use the in-tree provider, and tag the public subnets with kubernetes.io/role/elb=1."
	}
	if lbs.Err != nil {
		r.Status = StatusSkip
		r.Detail = fmt.Sprintf("Could not read Services: %v", lbs.Err)
		r.Fix = fix
		return r
	}
	var withAddress, pending []string
	for _, lb := range lbs.Items {
		name := lb.Namespace + "/" + lb.Name
		if lb.HasAddress {
			withAddress = append(withAddress, name)
		} else {
			pending = append(pending, name)
		}
	}
	switch {
	case len(withAddress) > 0:
		r.Status = StatusPass
		r.Detail = fmt.Sprintf("%d LoadBalancer Service(s) received an address, for example %s.", len(withAddress), withAddress[0])
	case len(pending) > 0:
		r.Status = StatusWarn
		r.Detail = fmt.Sprintf("LoadBalancer Service(s) without an address: %s. The Service that exposes your Qovery applications would stay without an address too.", joinEvidence(pending))
		r.Fix = fix
	default:
		r.Status = StatusSkip
		r.Detail = "No LoadBalancer Service on the cluster, so LoadBalancer support could not be confirmed."
		r.Fix = fix
	}
	return r
}

// metrics-server.
//
// Source: documentation byok.mdx, Prerequisites: services with autoscaling need the
// v1beta1.metrics.k8s.io API.
func checkMetricsServer(m MetricsAPIFacts) Result {
	r := Result{ID: "METRICS_SERVER_MISSING", Title: "metrics-server"}
	if m.Available {
		r.Status = StatusPass
		r.Detail = "The metrics.k8s.io API is available."
		return r
	}
	r.Status = StatusWarn
	r.Detail = "The metrics.k8s.io API is not available. Autoscaling of your services will not work."
	if m.Err != nil {
		r.Detail += fmt.Sprintf(" (%v)", m.Err)
	}
	r.Fix = "Install metrics-server. On EKS, add the metrics-server add-on. GKE and Kapsule include it."
	return r
}

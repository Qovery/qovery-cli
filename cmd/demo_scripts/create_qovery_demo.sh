#!/bin/sh

set -euo pipefail

CLUSTER_NAME=$1
ORGANIZATION_ID=$2
if [ "${3:0:4}" = "qov_" ]
then
  AUTHORIZATION_HEADER="Authorization: Token $3"
else
  AUTHORIZATION_HEADER="Authorization: Bearer $3"
fi

get_or_create_on_premise_account() {
  accountId=$(curl -s --fail-with-body -H "${AUTHORIZATION_HEADER}" -H 'Content-Type: application/json' https://api.qovery.com/organization/"${ORGANIZATION_ID}"/onPremise/credentials | jq -r .results[0].id)
  if [ "$accountId" = "null" ]
  then
    accountId=$(curl -s -X POST --fail-with-body -H "${AUTHORIZATION_HEADER}" -H 'Content-Type: application/json' -d '{"name": "on-premise"}' https://api.qovery.com/organization/"${ORGANIZATION_ID}"/onPremise/credentials | jq -r .id)
  fi

  echo "$accountId"
}

get_or_create_demo_cluster() {
  accountId=$1
  clusterName=$2
  clusterId=$(curl -s -X GET --fail-with-body -H "${AUTHORIZATION_HEADER}" -H 'Content-Type: application/json' https://api.qovery.com/organization/"${ORGANIZATION_ID}"/cluster | jq -r '.results[] | select(.name=="'"$clusterName"'") | .id')

  if [ "$clusterId" = "" ]
  then
    payload='{"name":"'$2'","region":"on-premise","cloud_provider":"ON_PREMISE","kubernetes":"SELF_MANAGED", "production": false,"features":[],"cloud_provider_credentials":{"cloud_provider":"ON_PREMISE","credentials":{"id":"'${accountId}'","name":"on-premise"},"region":"unknown"}}'
    clusterId=$(curl -s -X POST --fail-with-body -H "${AUTHORIZATION_HEADER}" -H 'Content-Type: application/json' -d "${payload}" https://api.qovery.com/organization/"${ORGANIZATION_ID}"/cluster | jq -r .id)
  fi

  echo "$clusterId"
}

get_cluster_values() {
  clusterId=$1
  curl -s -X GET --fail-with-body -H "${AUTHORIZATION_HEADER}" -H 'Content-Type: application/x-yaml' https://api.qovery.com/organization/"${ORGANIZATION_ID}"/cluster/"${clusterId}"/installationHelmValues
}

get_or_create_cluster() {
  clusterName=$1
  clusterExist=$(k3d cluster list -o json | jq '.[] | select(.name=="'"$clusterName"'") | .name')
  if [ "$clusterExist" = "" ]
  then
    k3d cluster create --k3s-arg "--disable=traefik@server:*" "$clusterName" --registry-create qovery-registry.lan
  else
    k3d cluster start "$clusterName"
  fi
}

install_or_upgrade_helm_charts() {
  releaseExist=$(helm list -n qovery -o json | jq '.[] | select(.name=="qovery") | .name')
  if [ "$releaseExist" = "" ]
  then
    set -x
    helm upgrade --install --create-namespace -n qovery -f values.yaml --atomic \
      --set services.certificates.cert-manager-configs.enabled=false \
      --set services.certificates.qovery-cert-manager-webhook.enabled=false \
      --set services.qovery.qovery-cluster-agent.enabled=false \
      --set services.qovery.qovery-engine.enabled=false \
      qovery qovery/qovery
  fi

  set -x
  helm upgrade --install --create-namespace -n qovery -f values.yaml --wait --atomic qovery qovery/qovery
  set +x
}

install_deps() {
  if which jq >/dev/null; then
     echo "jq already installed"
  else
    echo "jq command is missing. Please use your package manager to install it"
  fi

  if which grep >/dev/null; then
     echo "grep already installed"
  else
    echo "grep command is missing. Please use your package manager to install it"
  fi

  if which curl >/dev/null; then
     echo "curl already installed"
  else
    echo "curl command is missing. Please use your package manager to install it"
  fi

  if which k3d >/dev/null; then
    echo "k3d already installed"
  else
    echo "Installing k3d"
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=v5.6.3 bash
  fi

  if which helm >/dev/null; then
     echo "helm already installed"
  else
    echo "Installing HELM"
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  fi

  echo "All dependencies are installed"
}


echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Checking and installing dependencies'
echo '""""""""""""""""""""""""""""""""""""""""""""'
install_deps

accountId=$(get_or_create_on_premise_account)
clusterId=$(get_or_create_demo_cluster "${accountId}" "${CLUSTER_NAME}")

echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Fetching Qovery values to setup your cluster'
echo '""""""""""""""""""""""""""""""""""""""""""""'
get_cluster_values "${clusterId}" > values.yaml
echo "" >> values.yaml
curl -s -L https://raw.githubusercontent.com/Qovery/qovery-chart/main/charts/qovery/values-demo-local.yaml | grep -vE 'set-by-customer|^qovery:' >> values.yaml
echo 'Helm values written into values.yaml'


echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Installing Qovery helm repositories'
echo '""""""""""""""""""""""""""""""""""""""""""""'
helm repo add qovery https://helm.qovery.com
helm repo update

echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo "Creating $CLUSTER_NAME kube cluster"
echo '""""""""""""""""""""""""""""""""""""""""""""'
get_or_create_cluster "$CLUSTER_NAME"

echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Installing Qovery helm charts'
echo '""""""""""""""""""""""""""""""""""""""""""""'
install_or_upgrade_helm_charts


echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo "Qovery demo cluster is now installed !!!!"
echo "The kubeconfig is correctly set, so you can connect to it directly with kubectl or k9s from your local machine"
echo "To delete/stop/start your cluster, use k3d cluster xxxx"
echo ''
echo "Go to https://console.qovery.com to create your first environment on this cluster '${CLUSTER_NAME}'"
echo '""""""""""""""""""""""""""""""""""""""""""""'

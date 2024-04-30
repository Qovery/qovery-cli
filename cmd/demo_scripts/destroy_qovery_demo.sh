#!/bin/sh

set -eu

CLUSTER_NAME=$1
ORGANIZATION_ID=$2
#case $2 in
#  qov_*)
#    AUTHORIZATION_HEADER="Authorization: Token $3"
#  ;;
#
#  *)
#    AUTHORIZATION_HEADER="Authorization: Bearer $3"
#  ;;
#esac

delete_cluster() {
  clusterName=$1
  clusterExist=$(k3d cluster list -o json | jq '.[] | select(.name=="'"$clusterName"'") | .name')
  if [ -n "$clusterExist" ]
  then
    k3d cluster delete "$clusterName" || true
  fi
  docker network rm "k3d-${clusterName}" || true
}

teardown_network() {
  if [ "$(uname -s)" = 'Darwin' ]; then
    # MacOs
    set -x
    sudo ifconfig lo0 -alias 172.42.0.3/32 up || true
  elif grep -qi microsoft /proc/version; then
    # Wsl
    echo '******** PLEASE READ ********'
    echo 'You must run this command from an administrator terminal to finish the cleanup'
    echo 'netsh interface ipv4 delete address name="Loopback Pseudo-Interface 1" address=172.42.0.3'
    echo '******** PLEASE READ ********'
    set -x
    sudo ip addr del 172.42.0.3/32 dev lo || true
  fi
  set +x
}


echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Removing Qovery helm repositories'
echo '""""""""""""""""""""""""""""""""""""""""""""'
helm repo remove qovery || true

echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo "Removing $CLUSTER_NAME kube cluster"
echo '""""""""""""""""""""""""""""""""""""""""""""'
delete_cluster "$CLUSTER_NAME"

echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo 'Removing network config'
echo '""""""""""""""""""""""""""""""""""""""""""""'
teardown_network


echo ''
echo '""""""""""""""""""""""""""""""""""""""""""""'
echo "Qovery local demo cluster is now deleted"
echo "Your created environments still exits !"
echo "Go to https://console.qovery.com/organization/${ORGANIZATION_ID}/clusters/general to delete Qovery cluster config"
echo '""""""""""""""""""""""""""""""""""""""""""""'

#!/usr/bin/env bash

set -e
#set -x

echo "##################################"
echo "#                                #"
echo "#       QOVERY CLI INSTALL       #"
echo "#                                #"
echo "##################################"
echo ""

repo="Qovery/qovery-cli"
output_tgz="/tmp/qovery.tgz"
if ! command -v sudo 2>&1 >/dev/null
then
    dest_binary="."
else
    dest_binary="/usr/local/bin"
fi

os=$(uname | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  x86_64 | amd64)
    arch="amd64"
  ;;

  arm64 | aarch64)
    arch="arm64"
  ;;

  *)
    echo "Un-supported architecture. Please report to us to add it"
    exit 1
  ;;
esac

echo "[+] Downloading Qovery CLI archive..."

latest_tag=$(curl --silent "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
version=$(echo $latest_tag | sed 's/^v//')

test -f $output_tgz && rm -f $output_tgz
curl -o $output_tgz -sOL "https://github.com/${repo}/releases/download/${latest_tag}/qovery-cli_${version}_${os}_${arch}.tar.gz"

echo "[+] Uncompressing qovery binary in $dest_binary directory (sudo permissions are required)"
if ! command -v sudo 2>&1 >/dev/null
then
    tar -xzf $output_tgz -C $dest_binary qovery
else
    sudo tar -xzf $output_tgz -C $dest_binary qovery
fi
rm -f $output_tgz

echo -e "\nQovery CLI is installed, you can now use 'qovery' command line"

#!/usr/bin/env bash

set -e
#set -x

repo="Qovery/qovery-cli"
output_tgz="/tmp/qovery.tgz"
dest_binary="/usr/local/bin"
os=$(uname | tr '[:upper:]' '[:lower:]')

echo "Downloading Qovery CLI archive..."

latest_tag=$(curl --silent "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
version=$(echo $latest_tag | sed 's/^v//')

curl -o $output_tgz -sOL "https://github.com/${repo}/releases/download/${latest_tag}/qovery-cli_${version}_${os}_amd64.tar.gz"

echo "Uncompressing qovery binary in $dest_binary directory"
tar -xzf $output_tgz -C $dest_binary
rm -f $output_tgz

echo -e "\nQovery CLI is installed, you can now use 'qovery' command line"

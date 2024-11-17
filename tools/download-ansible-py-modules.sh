#!/bin/bash
set -e

if [ ! -d py_runtime ]; then
  echo "This script must be ran in the gosible source code root."
  exit 1
fi

rm -rf py_runtime/ansible

rm -rf ansible_source
mkdir -p ansible_source
cd ansible_source

ANSIBLE_VER=2.13.0rc1
curl -Lqo ansible_source.tar.gz https://github.com/ansible/ansible/archive/refs/tags/v$ANSIBLE_VER.tar.gz
tar -xf ansible_source.tar.gz ansible-$ANSIBLE_VER/lib/ansible/modules/ ansible-$ANSIBLE_VER/lib/ansible/module_utils/
mv ansible-*/lib/ansible ../py_runtime/ansible

cd ..
rm -rf ansible_source

cd py_runtime/ansible
patch -sp1 < ../patches/ansible_runtime_link_patch.txt
touch __init__.py



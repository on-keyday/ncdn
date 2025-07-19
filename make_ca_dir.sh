#!/bin/bash

if [ -z "$1" ]; then
  echo "Usage: $0 <ca_dir>"
  exit 1
fi

export MSYS_NO_PATHCONV=1
#mkdir -p $1/certs
#mkdir -p $1/crl
mkdir -p $1/newcerts
#mkdir -p $1/private
touch $1/index.txt
echo 1000 > $1/serial
#!/bin/bash
rc="$(godep save 2>&1)"
while echo $rc | grep "not found";
do
  package=$(echo $rc | sed -n "s/godep: Package (\(.*\)) not found/\1/p")
  go get $package
  rc="$(godep save 2>&1)"
done

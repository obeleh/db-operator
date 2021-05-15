#!/bin/bash
echo getting files in $1 
test_files=$(ls -l $1 | grep -v assert | grep -v delete | awk '{ print $9 }' | grep '-')
# echo $test_files
for i in $test_files
do
    echo $1/$i	
    kubectl apply -f $1/$i
done
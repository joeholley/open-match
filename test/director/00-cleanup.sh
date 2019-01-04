#!/bin/sh
set -x
kubectl delete $(kubectl get deploy om-director -o name)
kubectl delete $(kubectl get fleet -o name)
kubectl delete $(kubectl get fleetautoscaler -o name)
kubectl delete jobs --all

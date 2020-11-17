# Deployment Guide

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Deployment Guide](#deployment-guide)
    - [RBAC](#rbac)
        - [Deployment](#deployment)
    - [General process](#general-process)
<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## RBAC

The service account is `multicluster-operators-deployable`.

The role `multicluster-operators-deployable` is binded to that service account.

### Deployment

```shell
cd multicloud-operators-deployable
kubectl apply -f deploy/crds
kubectl apply -f deploy
```

## General process

Deployable CR:

```yaml
apiVersion: apps.open-cluster-management.io/v1
kind: Deployable
metadata:
  annotations:
    apps.open-cluster-management.io/is-local-deployable: "false"
  labels:
    deployable-label: "passed-in"
  name: example-configmap
  namespace: default
spec:
  template:
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: config1
      namespace: default
    data:
      purpose: for test
  placement:
    clusterSelector: {}
  overrides:
  - clusterName: endpoint2-ns
    clusterOverrides:
    - path: data
      value:
        foo: bar
```

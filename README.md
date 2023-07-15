redis-endpoint-operator
====================

Utility that manage kubernetes service endpoint,
reading current redis master IP address from redis sentinel service


Usage:

`/redis-endpoint-operator -sentinel IP:PORT -master NAME -service KUBE-SERVICE` 

Docker images available here: https://hub.docker.com/r/trublast/redis-endpoint-operator/tags

Usage example with [spotahome/redis-operator](https://github.com/spotahome/redis-operator) `v1.2.4`

```yaml
---
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: myproject
spec:
  sentinel:
    replicas: 3
    serviceAccountName: myproject-redis
    extraContainers:
    - name: endpoint-operator
      image: trublast/redis-endpoint-operator
      command:
        - /redis-endpoint-operator
        - -service
        - myproject-redis
  redis:
    replicas: 2
---
apiVersion: v1
kind: Service
metadata:
  name: myproject-redis
spec:
  ports:
  - port: 6379
    targetPort: 6379
    name: redis
---
apiVersion: v1
kind: Endpoints
metadata:
  name: myproject-redis
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: myproject-redis
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: myproject-redis
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myproject-redis
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: myproject-redis
subjects:
- kind: ServiceAccount
  name: myproject-redis
  namespace: {{ .Release.Namespace }}
```
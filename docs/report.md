
| 项目 | 值 |
|---|---|
| 扫描时间 | 2026-05-28 07:54:41 |
| Gerrit 实例 | `https://review.easystack.cn` |
| 扫描项目总数 | **692** |
| 扫描分支总数 | **3316** |
| 已迁移至 APISIX | **26** 个项目（32 个分支） |
| 未迁移（待重新扫描确认） | 需重新扫描 |




## 目录

- [已迁移至 APISIX 的项目](##已迁移至 APISIX 的项目)

- [未迁移至 APISIX 的项目](#安装)

- [使用方法](#使用方法)

- [FAQ](#faq)

## 已迁移至 APISIX 的项目

## easystack/ark

> 1 个分支已迁移 / 共 17 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `coaster/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `docker-registry/templates/ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `docker-registry/templates/registry-default-backend.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ems-dashboard/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ems-dashboard/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ems-dashboard/templates/ingress-ecp-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ems-dashboard/templates/ingress-ecp-dashboard-proxy.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ems-dashboard/templates/ingress-ecp-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `keystone/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `keystone/templates/ingress-sso.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ota/templates/ingress-ota-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ota/templates/ingress-ota-openapi.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `peak/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `roller-dashboard/templates/ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `coaster/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-barbican

> 1 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/barbican/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/barbican/templates/ingress-barbican-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/barbican/templates/ingress-barbican-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/barbican/templates/ingress-barbican-kms-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/barbican/templates/ingress-barbican-openapi.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/barbican/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-ceilometer

> 1 个分支已迁移 / 共 3 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/ceilometer/templates/aodh-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/ceilometer/templates/gnocchi-ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/ceilometer/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/ceilometer/templates/aodh-service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-ceph

> 1 个分支已迁移 / 共 11 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `ceph/templates/ingress-rgw-pluginconfig.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `ceph/templates/ingress-rgw.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `ceph/templates/service-ingress-rgw.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-cinder

> 1 个分支已迁移 / 共 8 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/cinder/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/cinder/templates/ingress-cinder-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/cinder/templates/ingress-cinder-golem.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/cinder/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-container-registry

> 1 个分支已迁移 / 共 6 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/container-registry/templates/ingress/ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/container-registry/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/container-registry/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |

</details>

## easystack/ark-devops

> 2 个分支已迁移 / 共 5 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard-api-noauth.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/jenkins-master-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/devops/templates/service-ingress-devops-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `stable/7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard-api-noauth.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/ingress-devops-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/devops/templates/jenkins-master-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/devops/templates/service-ingress-devops-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-drs

> 1 个分支已迁移 / 共 1 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/drs/templates/ingress-drs-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/drs/templates/ingress-drs-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/drs/templates/service-ingress-drs-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-ecms

> 1 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/ecms/templates/ecms-ingress-grpc.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/ecms/templates/ecms-ingress-web.yaml` | Ingress 使用 APISIX 作为 ingressClass |

</details>

## easystack/ark-ecns-appstore

> 1 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/ecns-appstore/templates/ingress-appstore-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/ecns-appstore/templates/ingress-appstore-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/ecns-appstore/templates/service-ingress-appstore-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-eks

> 1 个分支已迁移 / 共 8 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/eks/templates/ingress-api-eks.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/eks/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/eks/templates/ingress-eks-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/eks/templates/service-ingress-api-eks.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-eks-managed

> 1 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/eks-managed/templates/ingress-eks-managed-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/eks-managed/templates/ingress-eks-managed-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/eks-managed/templates/service-ingress-eks-managed-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-emla

> 1 个分支已迁移 / 共 5 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/emla/templates/emla-apiserver-grpc-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/emla/templates/emla-apiserver-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/emla/templates/emla-dashboard-api-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/emla/templates/emla-dashboard-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/emla/templates/emla-grafana-ingress.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/emla/templates/emla-apiserver-service-ingress.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-estack-dm

> 1 个分支已迁移 / 共 7 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/estack-dm/templates/ingress-gpu-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |

</details>

## easystack/ark-glance

> 1 个分支已迁移 / 共 7 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/glance/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/glance/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/glance/templates/ingress-glance-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/glance/templates/ingress-registry.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/glance/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-heat

> 2 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/heat/templates/ingress-api-vm.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cfn-vm.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cfn.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cloudwatch.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/heat/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `stable/7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/heat/templates/ingress-api-vm.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cfn-vm.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cfn.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-cloudwatch.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/heat/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/heat/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-horizon

> 1 个分支已迁移 / 共 7 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/horizon/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/horizon/templates/ingress-default-backend.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/horizon/templates/service-ingress.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-iam

> 1 个分支已迁移 / 共 7 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/iam/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/iam/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/iam/templates/ingress-hydra.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/iam/templates/ingress-iam-api-white-route.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/iam/templates/service-ingress-iam-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-multi-region

> 2 个分支已迁移 / 共 3 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/multi-region/templates/service-ingress-multi-region-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `stable/7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/multi-region/templates/ingress-multi-region-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/multi-region/templates/service-ingress-multi-region-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-murano

> 3 个分支已迁移 / 共 4 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/murano/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-murano-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/murano/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `stable/7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/murano/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-murano-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/murano/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `v7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/murano/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/murano/templates/ingress-murano-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/murano/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-nova

> 1 个分支已迁移 / 共 10 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/nova/templates/ingress-metadata.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/nova/templates/ingress-nova-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/nova/templates/ingress-nova-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/nova/templates/ingress-osapi.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/nova/templates/ingress-placement.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/nova/templates/service-ingress-metadata.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-octavia

> 2 个分支已迁移 / 共 5 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/octavia/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/octavia/templates/ingress-octavia-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/octavia/templates/ingress-octavia-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/octavia/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

### 分支 `stable/7.0.1`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/octavia/templates/ingress-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/octavia/templates/ingress-octavia-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/octavia/templates/ingress-octavia-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/octavia/templates/service-ingress-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-proton

> 1 个分支已迁移 / 共 8 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/proton/templates/ingress-proton-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/proton/templates/ingress-proton-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/proton/templates/ingress-server.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/proton/templates/service-ingress-neutron.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-schooner-arm

> 1 个分支已迁移 / 共 2 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/schooner-arm/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/schooner-arm/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/schooner-arm/templates/service-ingress-dashboard-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-schooner-x86

> 1 个分支已迁移 / 共 2 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/schooner-x86/templates/ingress-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/schooner-x86/templates/ingress-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/schooner-x86/templates/service-ingress-dashboard-api.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>

## easystack/ark-synapse

> 1 个分支已迁移 / 共 1 个分支

<details>
<summary>查看分支详情</summary>

### 分支 `master`

| 迁移方式 | 文件 | 详情 |
|---|---|---|
| IngressClass: apisix | `chart/synapse/templates/ingress-synapse-dashboard-api.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| IngressClass: apisix | `chart/synapse/templates/ingress-synapse-dashboard.yaml` | Ingress 使用 APISIX 作为 ingressClass |
| ExternalName: apisix | `chart/synapse/templates/service-ingress-synapse-dashboard.yaml` | 目标地址 `apisix-gateway.apisix.svc.cluster.local` |

</details>


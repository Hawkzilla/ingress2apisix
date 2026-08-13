# 流转图（Mermaid）

以下图表用于面试讲解与排障时「对齐心智模型」。细节以 `docs/design/architecture/` 为准。

## 1. 总体：CRI → Shim → VM → Agent

```mermaid
flowchart LR
  subgraph CP[Kubernetes 控制面]
    API[APIServer]
    Sched[Scheduler]
  end
  subgraph Node[工作节点]
    Kubelet[Kubelet]
    CRI[containerd 或 CRI-O]
    Shim[containerd-shim-kata-v2]
    VC[virtcontainers]
    HV[Hypervisor 进程]
    VFS[virtiofsd 等]
  end
  subgraph VM[轻量虚拟机]
    GK[Guest Kernel]
    AG[kata-agent ttRPC]
    CTR[容器 namespace/cgroup]
    WL[工作负载进程]
  end
  API --> Sched
  Sched --> Kubelet
  Kubelet --> CRI
  CRI --> Shim
  Shim --> VC
  VC --> HV
  VC --> VFS
  HV --> GK
  GK --> AG
  AG --> CTR
  CTR --> WL
```

## 2. Shim v2 调用关系（逻辑）

```mermaid
sequenceDiagram
  participant CD as containerd
  participant SH as shim-kata-v2
  participant VC as virtcontainers
  participant HV as Hypervisor
  participant AG as kata-agent
  CD->>SH: TaskService / shim v2 gRPC
  SH->>VC: CreateSandbox / 网络存储等
  VC->>HV: StartVM
  HV-->>AG: 启动 Guest 用户态 agent
  VC->>AG: ttRPC CreateSandbox / CreateContainer
  AG-->>VC: 响应 / IO 转发通道建立
  SH-->>CD: 任务状态、退出码
```

## 3. 容器创建（高层步骤）

```mermaid
flowchart TD
  A[用户 / Kubelet 请求创建] --> B[containerd 启动单个 shim 实例]
  B --> C[加载 configuration.toml]
  C --> D[shimv2 API 驱动 virtcontainers]
  D --> E[启动配置的超visor]
  E --> F[引导 Guest：kernel + guest image]
  F --> G[kata-agent 就绪]
  D --> H[ttRPC：CreateSandbox / CreateContainer]
  H --> I[Guest 内挂载 OCI bundle / virtio-fs]
  I --> J[启动工作负载]
```

## 4. Sandbox vs Pod 内容器（CRI-O 注解语义）

```mermaid
flowchart TD
  Q{OCI 注解 ContainerType?}
  Q -->|sandbox| N[新建 VM + 在 VM 内创建沙箱容器]
  Q -->|container| E[复用 Pod 已有 VM + 仅新建 VM 内容器]
```

对应官方文档中的决策表：`sandbox` → 创建 VM 与容器；`container` → 不新建 VM，仅在已有 VM 中加容器。

## 5. 关停路径（简化）

```mermaid
flowchart TD
  S1[工作负载退出] --> A1[agent 捕获退出码 WaitProcess]
  A1 --> R1[runtime 经 shim Wait 返回 containerd]
  R1 --> C1[回收 hypervisor / 网络 / 存储]
  S2[上层 Delete 强制删除] --> A2[runtime 发 DestroySandbox 等]
  A2 --> C1
```

若启用 agent tracing，关闭行为可能有额外阶段（见 `docs/tracing.md`）。

## 6. kata-deploy 在集群中的角色（安装视角）

```mermaid
flowchart LR
  Helm[Helm / kubectl] --> DS[DaemonSet kata-deploy]
  DS --> Node[各节点安装二进制与配置]
  DS --> RC[RuntimeClass 资源]
  DS --> CM[可选 ConfigMap / 运行时配置]
  subgraph Optional[可选集成]
    NFD[Node Feature Discovery]
    NFR[NodeFeatureRule CRD]
  end
  NFD --- NFR
  NFR -.->|节点标签 TEE 等| RC
```

# SRE\-Agent：基于LLM的K8s智能故障自愈系统

## 📌 项目简介

**SRE\-Agent** 是一款基于 Go、Client\-Go、Ollama 本地大模型、Prometheus、ArgoCD GitOps 构建的 **Kubernetes 智能故障自愈 AIOps 系统**。

传统 K8s 故障自愈多为**硬编码规则**（只要 Crash 就删 Pod），容易产生无效自愈、集群风暴。本项目引入**大模型智能决策**，让系统先“思考”再自愈，实现 **可解释、智能化、低误杀** 的自动化运维能力。

项目完整实现：**故障感知 → 上下文采集 → LLM 智能决策 → 自动化自愈执行** 的完整闭环。

## 🚀 技术栈

- **后端语言**：Golang

- **K8s 交互**：client\-go 官方 SDK

- **大模型服务**：Ollama（Qwen 2\.5 本地轻量化模型）

- **监控告警**：Prometheus \+ kube\-state\-metrics

- **持续部署**：ArgoCD GitOps

- **集群环境**：Kubernetes

- **部署方式**：声明式 GitOps 全自动部署

## 🎯 项目核心功能

1. **自动感知 K8s 故障**：定时拉取 Prometheus 告警，识别 `PodCrashLooping` 崩溃异常

2. **采集完整故障上下文**：自动获取 Pod 状态、容器日志、K8s 事件

3. **LLM 智能决策**：将故障信息交给本地大模型分析，判断是否适合重启自愈

4. **安全自愈执行**：仅支持固定动作 RESTART / IGNORE，杜绝大模型指令注入风险

5. **GitOps 全流程托管**：Ollama、监控组件、SRE\-Agent 全部由 ArgoCD 自动部署、同步、自愈

## 🔁 整体运行架构流程

**告警感知 → 信息采集 → LLM 推理决策 → 自愈执行**

1. **感知层**：Prometheus 采集集群指标，Pod 持续崩溃触发 `PodCrashLooping` 告警

2. **采集层**：Agent 通过 client\-go 获取 Pod 状态、最新日志、K8s 事件

3. **决策层**：构造 Prompt 传入 Ollama 大模型，由 AI 判断是否适合重启恢复

4. **执行层**：
        

   - AI 建议重启 → client\-go 删除 Pod，Deployment 自动重建实现自愈

   - AI 判断无效故障（配置错误、镜像错误）→ 忽略，避免无效自愈风暴

## 💡 项目亮点（面试/答辩核心加分点）

- **AI 决策与执行解耦（安全设计）**

  - 大模型 **不能直接执行命令**

  - 仅输出语义判断，程序通过关键词映射固定动作

  - 彻底杜绝 Prompt 注入、越权操作风险

- **区别于传统硬编码自愈**

  - 传统：只要崩溃就重启，无脑自愈

  - 本项目：AI 区分「临时抖动可恢复」和「配置错误不可恢复」

- **离线本地大模型**：基于 Ollama 本地推理，无需外网 API，稳定、安全、低延迟

- **标准 GitOps 工程化**：所有资源 YAML 入 Git，ArgoCD 统一管理，无集群配置漂移

- **高可扩展架构**：可继续扩展 OOM、镜像拉取失败、节点异常等多种自愈场景

## 📁 项目模块说明

- `getKubernetesClient`：初始化 K8s 客户端，支持集群内/集群外双环境

- `queryPrometheusAlerts`：拉取并过滤集群有效故障告警

- `getPodContext`：采集 Pod 日志、事件、运行状态，为 LLM 提供诊断依据

- `getLLMDecision`：组装 Prompt、调用 Ollama、解析 AI 自愈决策

- `executeAction`：执行自愈动作（删除 Pod / 忽略故障）

## ⚙️ 快速部署教程

### 1\. 环境依赖

- Kubernetes 集群 v1\.28\+

- ArgoCD 已安装

- Prometheus \+ kube\-state\-metrics

- Ollama 本地模型（qwen2\.5:1\.5b）

### 2\. 部署方式（GitOps）

将本项目 YAML 资源提交至 GitHub，由 ArgoCD 自动同步部署：

1. ArgoCD 关联 Git 仓库

2. 自动创建、同步、维护 Ollama / 监控 / SRE\-Agent 资源

3. 集群状态与 Git 始终保持一致，实现声明式运维

### 3\. 本地运行调试

```Plain Text
go mod tidy
go run main.go
```

## 🧪 测试方式

通过部署 `crash-app.yaml` 制造持续崩溃 Pod，触发 `CrashLoopBackOff` 故障：

```Plain Text
kubectl apply -f crash-app.yaml
```

Agent 自动识别故障、调用 AI 判断、选择性自愈。

## ⚠️ 项目不足与后续优化方向（面试必讲）

- 当前 LLM 解析基于关键词匹配，后续可改为 **JSON 结构化输出**，更加稳定

- 缺少**自愈冷却机制**，可增加内存 Map 防止短时间重复自愈引发集群风暴

- 目前仅支持 Pod Crash 故障，可扩展 OOM、镜像异常、节点压力等场景

- 未加入消息通知，后续可对接钉钉/企业微信推送自愈记录

- HTTP 请求未设置超时与重试，生产可增加健壮性优化

## 🧠 项目常见面试问答（内置答辩素材）

**Q：为什么不直接写死规则重启 Pod，要用 LLM？**

固定规则会无脑重启，遇到镜像错误、配置错误等问题会反复崩溃、反复重启，造成集群风暴。LLM 可以根据日志和事件判断根因，区分临时故障和永久故障，做到精准自愈。

**Q：删除 Pod 为什么能实现重启？**

K8s 没有重启 Pod API。Deployment 控制器负责维持副本数，删除 Pod 后控制器会自动新建 Pod，实现软重启，是生产最优实践。

**Q：如何保证 LLM 不会乱操作集群？**

项目做了严格解耦：LLM 只负责思考，不负责执行。所有可执行动作都是代码硬编码枚举，大模型无法下发任意指令，彻底规避注入风险。

**Q：Ollama 为什么需要持久化？**

模型文件较大，容器重建会丢失模型，每次重启重新拉取极慢。PVC 持久化保证模型永久留存。

## 📄 总结

本项目完成了 **云原生 \+ LLM AIOps 智能运维** 完整落地，将传统固定自动化运维升级为**可思考、可判断、低误杀**的智能自愈系统，适合云原生、SRE、后端开发项目展示。

> （注：部分内容可能由 AI 生成）

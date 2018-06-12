# 水晶桥(Crystal Bridge)

## 简介
水晶桥是一个非常轻量级的解决方案，主要用于桥接Kubernetes上POD所暴露的metrics信息，并将收集到的数据定期推送到远程所支持的Prometheus Push Gateway中。这个设计思路在很多方面都受到了eBay所提供的[Collectbeat](https://github.com/eBay/collectbeat)中部分功能的启发，但考虑到Metricbeat较为臃肿，并且将后端存储绑定到ElasticSearch并不会对Grafana的图表数据产生较好的用户体验，所以经过我的考虑，决定还是要将收集到的用户数据(Metrics)推送到Prometheus中进行统一的存储和数据计算。

### 技术思路概述
我们所期望的产品设计中，一个在Kubernetes上运行的程序，不仅仅需要收集其CPU、内存、网络以及I/O的容器性能数据，我们还要提供完整的解决方案用于收集和展示用户的自定义数据指标(Metrics)。Grafana的定位主要在图表展示上，但是其对于ES作为后端数据源的查询以及计算上，相比Prometheus体验很差。

通常上来讲，一个部署在Kubernetes上的程序，我们可以使用Prometheus通过对Kubernetes内部自己集成的cAdvisor进行数据收集。而在应用程序的自定义指标数据收集上，则需要水晶桥(Crystal Bridge)进行相关桥接工作。考虑到Prometheus Server主要采取拉的方式进行数据获取，本次我们的方案则还需要Prometheus Push Gateway的部署支持。即水晶桥(Crystal Bridge)在获取了用户程序所暴露的metrics数据后，将其主动推送到远端所部属的Prometheus Push Gateway中，然后对Prometheus Server进行配置，让其从这些固定部署的Push Gateway上进行数据收集即可。

而从用户容器程序暴露metrics接口的技术方案上，我们计划完全支持eBay在Collectbeat上提供的技术方案，采取动态发现资源Annotation的信息来进行数据获取的方案。所以说，水晶桥(Crystal Bridge)的技术方案相比Collectbeat或者Metricbeat都会简单很多，足够轻量。


  Name | Mandatory | Default Value | Description
  --- | --- | --- | ---
  `io.collectbeat.metrics/type` | Yes|  | What the format of the metrics being exposed is. Ex: `prometheus`, `dropwizard`, `mongodb`
  `io.collectbeat.metrics/endpoints` | Yes | | Comma separated locations to query the metrics from. Ex: `":8080/metrics, :9090/metrics"`
  `io.collectbeat.metrics/interval` | No | 1m | Time interval for metrics to be polled. Ex: `10m`, `1m`, `10s`
  `io.collectbeat.metrics/timeout` | No | 3s | Timeout duration for polling metrics. Ex: `10s`, `1m`
`io.collectbeat.metrics/namespace` | No | | Namespace to be provided for Dropwizard/Prometheus/HTTP metricsets.


水晶桥(Crystal Bridge)在部署上，依旧需要采取Daemonset的方式在Kubernetes集群中进行部署，在此项目完成后，我们会在github中直接给出Dockerfile以及部署到Kubernetes中所需要的Daemonset Yaml格式描述文件。

# 源代码管理方式
此项目采取[Git workflow](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow)的工作流分支管理方式，master分支永远保存已发布的最新release代码，develop分支用于保存活跃的开发版本，feature角色的分支主要用于开发新功能，等等，也请后续使用并跟进此项目的人知晓。

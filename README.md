**构建状态:** ![](https://travis-ci.org/g0194776/crystal-bridge.svg?branch=feature%2Ffirst_edition) 

**代码覆盖度:** [![Coverage Status](https://coveralls.io/repos/github/g0194776/crystal-bridge/badge.svg?branch=master)](https://coveralls.io/github/g0194776/crystal-bridge?branch=feature%2Ffirst_edition)


# 水晶桥(Crystal Bridge)

## 简介
水晶桥是一个非常轻量级的解决方案，主要用于桥接Kubernetes上POD所暴露的metrics信息，并将收集到的数据定期推送到远程所支持的Prometheus Push Gateway中。这个设计思路在很多方面都受到了eBay所提供的[Collectbeat](https://github.com/eBay/collectbeat)中部分功能的启发，但考虑到Metricbeat较为臃肿，并且将后端存储绑定到ElasticSearch并不会对Grafana的图表数据产生较好的用户体验，所以经过我的考虑，决定还是要将收集到的用户数据(Metrics)推送到Prometheus中进行统一的存储和数据计算。

### 技术思路概述
在我们所期望的产品设计中，一个在Kubernetes上运行的程序，不仅仅需要收集其CPU、内存、网络以及I/O的容器性能数据，我们还要提供完整的解决方案用于收集和展示用户的自定义数据指标(Metrics)。Grafana的定位主要在图表展示上，但是其对于ES作为后端数据源的查询以及计算上，相比Prometheus体验很差。

通常上来讲，一个部署在Kubernetes上的程序，我们可以使用Prometheus通过对Kubernetes内部自己集成的cAdvisor进行数据收集。而在应用程序的自定义指标数据收集上，则需要水晶桥(Crystal Bridge)进行相关桥接工作。考虑到Prometheus Server主要采取拉的方式进行数据获取，本次我们的方案则还需要Prometheus Push Gateway的部署支持。即水晶桥(Crystal Bridge)在获取了用户程序所暴露的metrics数据后，将其主动推送到远端所部属的Prometheus Push Gateway中，然后用户只需要对Prometheus Server进行配置，让其从这些固定部署的Push Gateway上进行数据收集即可。

而从用户容器程序暴露metrics接口的技术方案上，我们计划完全支持eBay在Collectbeat上所提供的技术方案，采取动态发现资源Annotation的信息来进行数据获取的方案。所以说，水晶桥(Crystal Bridge)的技术方案相比Collectbeat或者Metricbeat都会简单很多，足够轻量。

值得一提的是，除了POD级别Annotation的自动发现之外，水晶桥(Crystal Bridge)还将会额外完成一些更棒的工作。比如我们一直在考虑如何将Prometheus和Grafana联合起来，通过一种自动化的方式，将在Annotation中获取到的自定义数据指标动态地在远端Grafana中进行创建，这样一来用户便能够直接从Grafana中看到针对同样应用程序的两类图表信息:
- 基础性能数据信息
  - CPU
  - 内存
  - 网络
  - I/O
- 自定义业务指标数据

在针对POD级别的Annotation自动发现之后，水晶桥(Crystal Bridge)将会尝试对符合条件的POD Annotation进行自动并且持续的更新，加入其自定义业务指标数据解析后的相关信息，这样方式的元数据(Metadata)自动化注入工作，将会为后续自动在Grafana中创建图表进行直接的支持。注入的数据看起来类似如下的样子:

```text
Annotations:
io.auto-tagged.metrics-info=application_test_timer,SUMMARY;application_test1,GAUGE;application_test2,GAUGE;application_test3,GAUGE;application_test_histogram,SUMMARY;
io.collectbeat.metrics/endpoints=:30999/metrics
io.collectbeat.metrics/namespace=default
io.collectbeat.metrics/type=prometheus
```

```text


           +------------------------------------------------------------+
           |                                                            |
           |                                                            |
           |                        Prometheus                          |
           |                                                            |
           |                                                            |
           +------------------------------------------+-------------+---+
                                                      |             |
                                        Collect Metrics             |
                                                      |             |
                                                      |             |
          +---------------+  PUSH metrics   +---------v---------+   |
          |Crystal Bridge +----------------->Prometheus Push GW |   | Collect Container's
          +----+------+---+                 +-------------------+   | System Performance Metrics
               ^      |                                             |
               |      |                                             |
               |      |UPDATE                                       |
Pod Annotation |      |POD's Annotation                             |
Automatic Discovery   |                                             |
               |      |                                             |
           +---+------v---------------------------------------------v---+
           |                                                            |
           |                                                            |
           |                      Kubernetes Cluster                    |
           |                                                            |
           |                                                            |
           +------------------------------------------------------------+

```

水晶桥(Crystal Bridge)在部署上，依旧需要采取Daemonset的方式在Kubernetes集群中进行部署，在此项目完成后，我们会在github中直接给出Dockerfile以及部署到Kubernetes中所需要的Daemonset Yaml格式描述文件。

eBay Collectbeat所提供的Annotation字段详细描述如下:

  Name | Mandatory | Default Value | Description
  --- | --- | --- | ---
  `io.collectbeat.metrics/type` | Yes|  | What the format of the metrics being exposed is. Ex: `prometheus`, `dropwizard`, `mongodb`
  `io.collectbeat.metrics/endpoints` | Yes | | Comma separated locations to query the metrics from. Ex: `":8080/metrics, :9090/metrics"`
  `io.collectbeat.metrics/interval` | No | 1m | Time interval for metrics to be polled. Ex: `10m`, `1m`, `10s`
  `io.collectbeat.metrics/timeout` | No | 3s | Timeout duration for polling metrics. Ex: `10s`, `1m`
`io.collectbeat.metrics/namespace` | No | | Namespace to be provided for Dropwizard/Prometheus/HTTP metricsets.

# 源代码管理方式
此项目采取[Git workflow](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow)的工作流分支管理方式，master分支永远保存已发布的最新release代码，develop分支用于保存活跃的开发版本，feature角色的分支主要用于开发新功能，等等，也请后续使用并跟进此项目的人知晓。

# 如何编译此项目?
我们使用`godep`工具来对此项目做依赖包管理，请使用如下脚本进行godep的包还原以及项目编译工作。

```shell
godep restore -v && go build
```

如果你还没有安装`godep`，请[点击这里](https://github.com/tools/godep)进入godep主页，以便进行安装。代码根目录下的`find_missed_packages_for_godep.sh`文件主要用于解决第一次使用godep工具，而无法正常执行godep save命令的问题。

如果你也遇到了此类问题，请尝试使用命令行执行此文件，以便自动下载所有godep所需要的并且缺失的包。
```shell
./find_missed_packages_for_godep.sh
```

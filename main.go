package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
)

func main() {
	args = initializeArg()
	ch := initializeK8SInformer()
	resultChan := initKubernetesPODEventProcessor(ch)
	initializePrometheusPusher(resultChan)
	fmt.Println("Crystal Bridge has been started successfully!")
	select {} //block current process.
}

var (
	args *CommandLineArgs
)

func initializeArg() *CommandLineArgs {
	arg := CommandLineArgs{}
	flag.IntVar(&arg.LogLevel, "l", 2, "log level.")
	flag.StringVar(&arg.RemotePrometheusPushGWAddr, "gw", "", "the accessabile address of remote prometheus push gateway.")
	flag.StringVar(&arg.AnnotationPrefixTag, "tag", "io.collectbeat.metrics", "a prefix value used for matching POD's annotations.")
	flag.IntVar(&arg.PrometheusDataSyncBufferSize, "syncbuffer", 32, "length of buffered queue size for syncing data to the remote Prometheus push gateway")
	flag.StringVar(&arg.Host, "host", "", "hostname, usually be set as current machine's IP address.")
	flag.StringVar(&arg.FechingInterval, "fi", "1m", "fetching interval")
	flag.StringVar(&arg.FechingTimeout, "ft", "3s", "fetching timeout")
	flag.StringVar(&arg.LabeledNamespace, "lns", "3s", "labeled namespace on the POD's annotation.")
	flag.StringVar(&arg.KubernetesAddress, "k8saddr", "", "remote Kubernetes URL. e.g. http://xxx.xxx.xxx.xxx:8080")
	flag.StringVar(&arg.KubernetesBearerToken, "k8sbt", "", "Kubernetes bearer token")
	flag.Parse()

	fmt.Println("Initializing logger...")
	//minimum level to log.
	log.SetLevel(log.Level(arg.LogLevel))
	return &arg
}

type CommandLineArgs struct {
	LogLevel                   int
	RemotePrometheusPushGWAddr string
	//current machine's hostname (IP ADDRESS)
	Host                         string
	AnnotationPrefixTag          string
	FechingInterval              string
	FechingTimeout               string
	LabeledNamespace             string
	KubernetesAddress            string
	KubernetesBearerToken        string
	PrometheusDataSyncBufferSize int
}

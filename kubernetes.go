package main

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"strings"
)

const (
	POD_ADD PODStatus = iota
	POD_UPDATE
	POD_DELETE
)

var (
	k8sClient *kubernetes.Clientset
	eventChan chan *PODEvent
)

type PODStatus int
type PODEvent struct {
	Pod              *corev1.Pod
	Status           PODStatus
	MetricType       string
	Endpoints        string
	FechingInterval  string
	FechingTimeout   string
	LabeledNamespace string
	HasAnnotation    bool
}

func (e *PODEvent) ParseAnnotation() {
	if e.Pod.Annotations != nil && len(e.Pod.Annotations) > 0 {
		//e.g. io.collectbeat.metrics/type
		if metricType, ok := e.Pod.Annotations[args.AnnotationPrefixTag+"/type"]; ok {
			//ONLY "prometheus" can be supported.
			if strings.ToLower(metricType) != "prometheus" {
				log.Warnf("Skipped POD: %s which has been set to an unsupported type of metric type(%s)", e.Pod.Name, metricType)
				return
			}
			//e.g. io.collectbeat.metrics/endpoints
			if eps, ok := e.Pod.Annotations[args.AnnotationPrefixTag+"/endpoints"]; ok {
				e.MetricType = metricType
				e.Endpoints = eps
				e.HasAnnotation = true
				//try to detect fetching interval.
				if interval, ok := e.Pod.Annotations[args.AnnotationPrefixTag+"/interval"]; ok {
					e.FechingInterval = interval
				} else {
					e.FechingInterval = args.FechingInterval
				}
				//try to detect fetching timeout.
				if timeout, ok := e.Pod.Annotations[args.AnnotationPrefixTag+"/timeout"]; ok {
					e.FechingTimeout = timeout
				} else {
					e.FechingTimeout = args.FechingTimeout
				}
				//try to detect labeled namespace.
				if ns, ok := e.Pod.Annotations[args.AnnotationPrefixTag+"/namespace"]; ok {
					e.LabeledNamespace = ns
				} else {
					e.LabeledNamespace = args.LabeledNamespace
				}
			}
		}
	} else {
		e.HasAnnotation = false
	}
}

func initializeK8SInformer() chan *PODEvent {
	log.Infoln("Initializing Kubernetes informer...")
	var err error
	k8sClient, err = kubernetes.NewForConfig(&rest.Config{Host: args.KubernetesAddress, BearerToken: args.KubernetesBearerToken})
	if err != nil {
		log.Panicf("CANNOT init Kubernetes client, error: %s", err.Error())
	}
	sharedFactory := informers.NewSharedInformerFactory(k8sClient, 0)
	sharedFactory.WaitForCacheSync(make(chan struct{}))
	log.Infoln("Fully synchronizing PODs...")
	eventChan = make(chan *PODEvent, 256)
	go syncPods()
	return eventChan
}

func syncPods() {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = "spec.nodeName=" + args.Host
				return k8sClient.CoreV1().Pods("").List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return k8sClient.CoreV1().Pods("").Watch(options)
			},
		},
		&corev1.Pod{},
		0, //Skip resyncr
		cache.Indexers{},
	)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Debugf("informer ADD event received: %s", obj.(*corev1.Pod).Name)
			handlePodModify(obj.(*corev1.Pod), POD_ADD)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if oldObj.(*corev1.Pod).String() == newObj.(*corev1.Pod).String() {
				//NOT REALLY CHANGED.
				return
			}
			log.Debugf("informer UPDATE event received: %s", newObj.(*corev1.Pod).Name)
			handlePodModify(newObj.(*corev1.Pod), POD_UPDATE)
		},
		DeleteFunc: func(obj interface{}) {
			log.Debugf("informer DELETE event received: %s", obj.(*corev1.Pod).Name)
			handlePodModify(obj.(*corev1.Pod), POD_DELETE)
		},
	})
	go informer.Run(make(chan struct{}) /*ignored stop signal.*/)
}

func handlePodModify(pod *corev1.Pod, status PODStatus) {
	pe := &PODEvent{Status: status, Pod: pod}
	//try parsing annotation.
	pe.ParseAnnotation()
	log.Debugf("POD event received: %#v", pe)
	//directly send it to the channel without any filtering steps.
	eventChan <- pe
}

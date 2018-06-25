package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	automaticTaggedAnnotationKey = "io.auto-tagged.metrics-info"
)

var (
	prometheusOutputChan chan *PrometheusData
	monitoringPods       map[types.UID]*PODMetricsMonitor
	lock                 *sync.Mutex
	fetchSucceedCounter  = prometheus.NewCounter(prometheus.CounterOpts{Name: "fetch_prometheus_metrics_succeed_count_total", Help: "Total count of successful fetch the remote Prometheus metric endpoints."})
	fetchFailedCounter   = prometheus.NewCounter(prometheus.CounterOpts{Name: "fetch_prometheus_metrics_failed_count_total", Help: "Total count of failed fetching the remote Prometheus metric endpoints."})
)

type PrometheusData struct {
	RspData      []byte
	FetchingTime time.Time
	ResourceName string
	PodName      string
	PodIP        string
	HostIP       string
	Namespace    string
	NeedDelete   bool
}

type PODMetricsMonitor struct {
	Event  PODEvent
	Ctx    context.Context //used for cancellation.
	Cancel func()
	client *http.Client
}

func (m *PODMetricsMonitor) Start() {
	if m.client == nil {
		duration, err := time.ParseDuration(m.Event.FechingTimeout)
		if err != nil {
			log.Panicf("Failed to parse formatted timeout string for fetching the remote metrics.")
		}
		m.client = &http.Client{Timeout: duration, Transport: &http.Transport{MaxIdleConns: 10, TLSHandshakeTimeout: 0}}
	}
	go func() {
		duration, err := time.ParseDuration(m.Event.FechingInterval)
		if err != nil {
			log.Panicf("Failed to parse formatted duration string: %s", m.Event.FechingInterval)
		}
		timeChan := time.Tick(duration)
		for {
			select {
			case <-m.Ctx.Done():
				m.client = nil
				return
			case <-timeChan:
				doFetch(m)
			}
		}
	}()
}

func doFetch(m *PODMetricsMonitor) {
	url := fmt.Sprintf("http://%s%s", m.Event.Pod.Status.PodIP, m.Event.Endpoints)
	log.Debugf("%#v", m.Event.Pod.Status)
	log.Debugf("Preparing to fetch metrics URL: %s, POD IP: %s", url, m.Event.Pod.Status.PodIP)
	//m.Event.Endpoints support ONLY one address by now.
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fetchFailedCounter.Inc()
		log.Errorf("[Fetching Metric] Failed to fetch POD's metric, error: %s", err.Error())
		return
	}
	rsp, err := m.client.Do(req)
	if err != nil {
		fetchFailedCounter.Inc()
		log.Errorf("[Fetching Metric] Failed to fetch POD's metric, error: %s", err.Error())
		return
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		fetchFailedCounter.Inc()
		log.Errorf("[Fetching Metric] Failed to fetch POD's metric, HTTP response status code: %d", rsp.StatusCode)
		return
	}
	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		fetchFailedCounter.Inc()
		log.Errorf("[Fetching Metric] Failed to read HTTP response body: %s", err.Error())
		return
	}
	fetchSucceedCounter.Inc()
	sendMessage(&m.Event, data, false)
}

func initKubernetesPODEventProcessor(eventChan chan *PODEvent) chan *PrometheusData {
	log.Infoln("Initializing Kubernetes POD's event processor...")
	prometheus.MustRegister(fetchSucceedCounter)
	prometheus.MustRegister(fetchFailedCounter)
	http.Handle("/metrics", prometheus.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(":36000", nil))
	}()
	if prometheusOutputChan == nil {
		prometheusOutputChan = make(chan *PrometheusData, args.PrometheusDataSyncBufferSize)
	}
	lock = &sync.Mutex{}
	monitoringPods = make(map[types.UID]*PODMetricsMonitor)
	go readPodEvents(eventChan)
	return prometheusOutputChan
}

func readPodEvents(eventChan chan *PODEvent) {
	for e := range eventChan {
		processPodEvent(e)
	}
}

func processPodEvent(e *PODEvent) {
	if e.Status == POD_ADD && !e.HasAnnotation {
		return
	}
	lock.Lock()
	defer lock.Unlock()
	if monitor, ok := monitoringPods[e.Pod.UID]; ok {
		if e.Status == POD_DELETE {
			delete(monitoringPods, e.Pod.UID)
			monitor.Cancel()
			//try removing remote persisted Prometheus metrics.
			sendMessage(e, nil, true)
		} else if e.Status == POD_UPDATE {
			//never exposed any metric endpoints, close it.
			if !e.HasAnnotation {
				delete(monitoringPods, e.Pod.UID)
				monitor.Cancel()
				//try removing remote persisted Prometheus metrics.
				sendMessage(e, nil, true)
				return
			}
			//annotation updated, try restarting it.
			if isAnnotationChanged(&monitor.Event, e) {
				monitor.Cancel()
				monitor.Event = *e
				monitor.Start()
				return
			}
		}
	} else {
		if e.Status == POD_ADD || (e.Status == POD_UPDATE && e.HasAnnotation) {
			if e.Pod.Status.PodIP == "" {
				log.Debugf("Ignored POD \"%s\" without any IP.", e.Pod.Name)
				return
			}
			ctx, cancel := context.WithCancel(context.Background())
			pmm := &PODMetricsMonitor{Event: *e, Ctx: ctx, Cancel: cancel}
			monitoringPods[e.Pod.UID] = pmm
			pmm.Start()
		}
	}
}
func isAnnotationChanged(old *PODEvent, new *PODEvent) bool {
	if old.MetricType != new.MetricType {
		return true
	}
	if old.Endpoints != new.Endpoints {
		return true
	}
	if old.FechingInterval != new.FechingInterval {
		return true
	}
	if old.FechingTimeout != new.FechingTimeout {
		return true
	}
	if old.LabeledNamespace != new.LabeledNamespace {
		return true
	}
	return false
}

func sendMessage(e *PODEvent, data []byte, needDelete bool) {
	kind, name, ns, err := retrievePodInformation(e.Pod)
	if err != nil {
		log.Errorf("Failed to retrieve POD's resource metadata (%s), error: %s", e.Pod.Name, err.Error())
	}
	obj := &PrometheusData{
		RspData:      data,
		FetchingTime: time.Now(),
		ResourceName: fmt.Sprintf("%s_%s_%s", ns, kind, name),
		PodName:      e.Pod.Name,
		PodIP:        e.Pod.Status.PodIP,
		HostIP:       e.Pod.Status.HostIP,
		Namespace:    e.Pod.Namespace,
		NeedDelete:   needDelete}
	needUpdate, err := needUpdateAnnotation(e, obj)
	if err != nil {
		log.Errorf("Failed to parse Prometheus metric metadata, POD: %s, error: %s", e.Pod.Name, err.Error())
	}
	if needUpdate {
		updatePod(e)
	}
	prometheusOutputChan <- obj
}

func needUpdateAnnotation(e *PODEvent, obj *PrometheusData) (bool, error) {
	var err error
	parser := expfmt.TextParser{}
	metrics, err := parser.TextToMetricFamilies(bytes.NewReader(obj.RspData))
	if err != nil {
		return false, err
	}
	sb := strings.Builder{}
	for _, v := range metrics {
		sb.WriteString(*v.Name)
		sb.WriteString(",")
		sb.WriteString(v.Type.String())
		sb.WriteString(";")
	}
	oldAnnotatedStr := e.Pod.Annotations[automaticTaggedAnnotationKey]
	newAnnotatedStr := sb.String()
	if oldAnnotatedStr != newAnnotatedStr {
		e.NeededAppendingAnnotation = newAnnotatedStr
		return true, nil
	}
	return false, nil
}

type Annotation struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Reference  struct {
		Kind            string `json:"kind"`
		Namespace       string `json:"namespace"`
		Name            string `json:"name"`
		UID             string `json:"uid"`
		APIVersion      string `json:"apiVersion"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"reference"`
}

func retrievePodInformation(pod *corev1.Pod) (string, string, string, error) {
	var refer Annotation
	var kind, name, ns string
	//兼容Kubernetes v1.9.x版本对象结构
	if _, ok := pod.Annotations["kubernetes.io/created-by"]; !ok {
		if pod.ObjectMeta.OwnerReferences == nil || len(pod.ObjectMeta.OwnerReferences) == 0 {
			return "", "", "", fmt.Errorf("Could not retrieve any metadata from given Pod: %s", pod.Name)
		}
		kind = pod.ObjectMeta.OwnerReferences[0].Kind
		name = pod.ObjectMeta.OwnerReferences[0].Name
		ns = pod.Namespace
	} else {
		//老版本Kubernetes对象结构处理
		err := json.Unmarshal([]byte(pod.Annotations["kubernetes.io/created-by"]), &refer)
		if err != nil {
			return "", "", "", err
		}
		kind = refer.Reference.Kind
		name = refer.Reference.Name
		ns = refer.Reference.Namespace
	}
	return kind, name, ns, nil
}

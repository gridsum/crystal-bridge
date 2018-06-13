package main

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

var (
	prometheusOutputChan chan *PrometheusData
	monitoringPods       map[types.UID]*PODMetricsMonitor
	lock                 *sync.Mutex
)

type PrometheusData struct {
}

type PODMetricsMonitor struct {
	Event  PODEvent
	Cancel func()
}

func (m *PODMetricsMonitor) Start() {

}

func initKubernetesPODEventProcessor(eventChan chan *PODEvent) chan *PrometheusData {
	log.Infoln("Initializing Kubernetes POD's event processor...")
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
		} else if e.Status == POD_UPDATE {
			//never exposed any metric endpoints, close it.
			if !e.HasAnnotation {
				delete(monitoringPods, e.Pod.UID)
				monitor.Cancel()
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
			pmm := &PODMetricsMonitor{Event: *e}
			monitoringPods[e.Pod.UID] = pmm
			pmm.Start()
		}
	}
}
func isAnnotationChanged(old *PODEvent, new *PODEvent) bool {
	return false
}

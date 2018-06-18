package main

import (
	"context"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sync"
	"time"
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
			pmm := &PODMetricsMonitor{Event: *e, Ctx: context.Background()}
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

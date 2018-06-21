package main

import (
	"bytes"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

var (
	pushGWClient       *http.Client
	pushSucceedCounter = prometheus.NewCounter(prometheus.CounterOpts{Name: "push_prometheus_metrics_succeed_count_total", Help: "Total count of successfully push the remote Prometheus metric endpoints."})
	pushFailedCounter  = prometheus.NewCounter(prometheus.CounterOpts{Name: "push_prometheus_metrics_failed_count_total", Help: "Total count of failed pushing the remote Prometheus metric endpoints."})
)

func initializePrometheusPusher(data chan *PrometheusData) {
	log.Infoln("Initializing Prometheus push GW proxy...")
	prometheus.MustRegister(pushSucceedCounter)
	prometheus.MustRegister(pushFailedCounter)
	duration, err := time.ParseDuration(args.RemotePrometheusPushGWAddrHttpTimeout)
	if err != nil {
		log.Panicf("Failed to parse GW push timeout value to type of time.duration, err: %s", err.Error())
	}
	pushGWClient = &http.Client{
		Timeout:   duration,
		Transport: &http.Transport{MaxIdleConns: 10, TLSHandshakeTimeout: 0}}
	go readMessage(data)
}

func readMessage(data chan *PrometheusData) {
	var err error
	for msg := range data {
		if !msg.NeedDelete { //PUSH metric data to GW
			err = pushDataToGW(msg)
			if err != nil {
				log.Errorf("Failed to push data to the remote Prometheus GW, error: %s", err.Error())
			}
		} else {
			err = deletePrometheusMetric(msg)
			if err != nil {
				log.Errorf("Failed to remove remote Prometheus metric, error: %s", err.Error())
			} else {
				log.Infof("Metrics for POD: %s has been deleted successfully.", msg.PodName)
			}
		}
	}
}

func pushDataToGW(data *PrometheusData) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/metrics/job/%s/instance/%s", args.RemotePrometheusPushGWAddr, data.ResourceName, data.PodName), bytes.NewReader(data.RspData))
	if err != nil {
		pushFailedCounter.Inc()
		return err
	}
	rsp, err := pushGWClient.Do(req)
	if err != nil {
		pushFailedCounter.Inc()
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusAccepted {
		pushFailedCounter.Inc()
		return fmt.Errorf("HTTP RSP status-code: %d", rsp.StatusCode)
	}
	pushSucceedCounter.Inc()
	return nil
}

func deletePrometheusMetric(data *PrometheusData) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://%s/metrics/job/%s/instance/%s", args.RemotePrometheusPushGWAddr, data.ResourceName, data.PodName), nil)
	if err != nil {
		return err
	}
	rsp, err := pushGWClient.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("HTTP RSP status-code: %d", rsp.StatusCode)
	}
	return nil
}

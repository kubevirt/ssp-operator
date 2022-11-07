package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

const (
	totalRejectedVmsMetricsValue               = "total_rejected_vms"
	totalRestoredCommonTemplatesMetricsValue   = "total_restored_common_templates"
	sspOperatorReconcilingProperlyMetricsValue = "ssp_operator_reconciling_properly"
)

var regexpForMetrics = map[string]*regexp.Regexp{
	totalRejectedVmsMetricsValue:               regexp.MustCompile(`total_rejected_vms ([0-9]+)`),
	totalRestoredCommonTemplatesMetricsValue:   regexp.MustCompile(`total_restored_common_templates ([0-9]+)`),
	sspOperatorReconcilingProperlyMetricsValue: regexp.MustCompile(`ssp_operator_reconciling_properly ([0-9]+)`),
}

func intMetricValue(metricName string, metricsPort uint16, pod *v1.Pod) (int, error) {
	conn, err := portForwarder.Connect(pod, metricsPort)
	if err != nil {
		return 0, fmt.Errorf("could not connect port-forwarder: %w", err)
	}
	defer conn.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return conn, nil
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/metrics", metricsPort))
	if err != nil {
		return 0, fmt.Errorf("could not get metrics: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("could not read response body: %w", err)
	}

	regex, ok := regexpForMetrics[metricName]
	if !ok {
		panic(fmt.Sprintf("metricName %s does not have a defined regexp string, please add one to the regexpForMetrics map", metricName))
	}
	valueOfMetric := regex.FindSubmatch(body)
	intValue, err := strconv.Atoi(string(valueOfMetric[1]))
	if err != nil {
		return 0, fmt.Errorf("could not parse metric int value: %w", err)
	}

	return intValue, nil
}

func intMetricValuePods(metricName string, metricsPort uint16, pods []v1.Pod) (int, error) {
	var sum int
	for i := range pods {
		value, err := intMetricValue(metricName, metricsPort, &pods[i])
		if err != nil {
			return 0, fmt.Errorf("error getting metric value from pod %s/%s: %w", pods[i].Namespace, pods[i].Name, err)
		}
		sum += value
	}
	return sum, nil
}

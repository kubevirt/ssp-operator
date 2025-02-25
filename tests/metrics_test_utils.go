package tests

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

var regexpForMetrics = map[string]*regexp.Regexp{
	"kubevirt_ssp_template_validator_rejected_total": regexp.MustCompile(`kubevirt_ssp_template_validator_rejected_total ([0-9]+)`),
	"kubevirt_ssp_common_templates_restored_total":   regexp.MustCompile(`kubevirt_ssp_common_templates_restored_total ([0-9]+)`),
	"kubevirt_ssp_operator_reconcile_succeeded":      regexp.MustCompile(`kubevirt_ssp_operator_reconcile_succeeded ([0-9]+)`),
}

func intMetricValue(metricName string, metricsPort uint16, pod *v1.Pod) (value int, err error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return portForwarder.Connect(pod, metricsPort)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			// We don't want to reuse port-forward connections for multiple requests.
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/metrics", metricsPort))
	if err != nil {
		return 0, fmt.Errorf("failed to get metrics from %s: %w", pod.Name, err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read metrics response body: %w", err)
	}

	regex, ok := regexpForMetrics[metricName]
	if !ok {
		panic(fmt.Sprintf("metricName %s does not have a defined regexp string, please add one to the regexpForMetrics map", metricName))
	}

	valueOfMetric := regex.FindSubmatch(body)
	intValue, err := strconv.Atoi(string(valueOfMetric[1]))
	if err != nil {
		return 0, fmt.Errorf("failed to convert metric %s value to int: %w", metricName, err)
	}

	return intValue, nil
}

package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var regexpForMetrics = map[string]*regexp.Regexp{
	"kubevirt_ssp_template_validator_rejected_total": regexp.MustCompile(`kubevirt_ssp_template_validator_rejected_total ([0-9]+)`),
	"kubevirt_ssp_common_templates_restored_total":   regexp.MustCompile(`kubevirt_ssp_common_templates_restored_total ([0-9]+)`),
	"kubevirt_ssp_operator_reconcile_succeeded":      regexp.MustCompile(`kubevirt_ssp_operator_reconcile_succeeded ([0-9]+)`),
}

func intMetricValue(metricName string, metricsPort uint16, pod *v1.Pod) int {
	conn, err := portForwarder.Connect(pod, metricsPort)
	Expect(err).ToNot(HaveOccurred())
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
	Expect(err).ToNot(HaveOccurred(), "Can't get metrics from %s", pod.Name)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	regex, ok := regexpForMetrics[metricName]
	if !ok {
		panic(fmt.Sprintf("metricName %s does not have a defined regexp string, please add one to the regexpForMetrics map", metricName))
	}
	valueOfMetric := regex.FindSubmatch(body)
	intValue, err := strconv.Atoi(string(valueOfMetric[1]))
	Expect(err).ToNot(HaveOccurred())
	return intValue
}

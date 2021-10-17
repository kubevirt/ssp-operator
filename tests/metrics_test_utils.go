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

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var regexpForMetrics = map[string]*regexp.Regexp{
	"total_rejected_vms":              regexp.MustCompile(`total_rejected_vms ([0-9]+)`),
	"total_restored_common_templates": regexp.MustCompile(`total_restored_common_templates ([0-9]+)`),
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
	body, _ := ioutil.ReadAll(resp.Body)
	regex, ok := regexpForMetrics[metricName]
	if !ok {
		panic(fmt.Sprintf("metricName %s does not have a defined regexp string, please add one to the regexpForMetrics map", metricName))
	}
	valueOfMetric := regex.FindSubmatch(body)
	intValue, err := strconv.Atoi(string(valueOfMetric[1]))
	Expect(err).ToNot(HaveOccurred())
	return intValue
}

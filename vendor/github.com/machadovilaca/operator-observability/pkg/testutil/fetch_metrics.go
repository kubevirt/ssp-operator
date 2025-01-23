package testutil

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type metricQuery struct {
	url           string
	metricName    string
	labelKeyValue map[string]string
}

type MetricResult struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

func FetchMetric(URL string, metricName string, labelsKeyValue ...string) ([]MetricResult, error) {
	labelFilters, err := buildLabelsFilter(labelsKeyValue...)
	if err != nil {
		return nil, err
	}

	mq := metricQuery{
		url:           URL,
		metricName:    metricName,
		labelKeyValue: labelFilters,
	}

	resp, err := http.Get(URL)
	if err != nil {
		return nil, fmt.Errorf("failed to query service endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	var results []MetricResult
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		mr, found, lineErr := mq.parseLine(line)
		if lineErr != nil {
			return nil, err
		}

		if found {
			results = append(results, *mr)
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", scanErr)
	}

	return results, nil
}

func buildLabelsFilter(labelsKeyValue ...string) (map[string]string, error) {
	labelFilters := make(map[string]string)
	for i := 0; i < len(labelsKeyValue); i += 2 {
		// Ensure that we have a key-value pair
		if !(i+1 < len(labelsKeyValue)) {
			return nil, fmt.Errorf("invalid label key-value pair: %s", labelsKeyValue[i])
		}

		labelFilters[labelsKeyValue[i]] = labelsKeyValue[i+1]
	}

	return labelFilters, nil
}

func (mq *metricQuery) parseLine(line string) (*MetricResult, bool, error) {
	// Ignore comments and empty lines
	if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		return nil, false, nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, false, fmt.Errorf("invalid metric line: %s", line)
	}
	nameAndLabels := parts[0]
	valueStr := parts[1]

	name, labels := parseMetricNameAndLabels(nameAndLabels)
	if name != mq.metricName {
		return nil, false, nil
	}

	if !matchLabels(labels, mq.labelKeyValue) {
		return nil, false, nil
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse metric value: %w", err)
	}

	return &MetricResult{
		Name:   name,
		Labels: labels,
		Value:  value,
	}, true, nil
}

func parseMetricNameAndLabels(input string) (string, map[string]string) {
	labels := make(map[string]string)
	nameEnd := strings.Index(input, "{")
	if nameEnd == -1 {
		return input, labels
	}

	name := input[:nameEnd]
	labelStr := strings.TrimSuffix(strings.TrimPrefix(input[nameEnd:], "{"), "}")
	labelPairs := strings.Split(labelStr, ",")
	for _, pair := range labelPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = strings.Trim(kv[1], "\"")
		}
	}

	return name, labels
}

func matchLabels(metricLabels, filters map[string]string) bool {
	for k, v := range filters {
		if metricLabels[k] != v {
			return false
		}
	}
	return true
}

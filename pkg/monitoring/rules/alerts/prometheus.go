package alerts

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
)

const (
	prometheusRunbookAnnotationKey = "runbook_url"
	partOfAlertLabelKey            = "kubernetes_operator_part_of"
	partOfAlertLabelValue          = "kubevirt"
	componentAlertLabelKey         = "kubernetes_operator_component"
	componentAlertLabelValue       = "ssp-operator"
	defaultRunbookURLTemplate      = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv          = "RUNBOOK_URL_TEMPLATE"
)

func Register(registry *operatorrules.Registry) error {
	alerts := operatorAlerts()

	runbookURLTemplate, err := getRunbookURLTemplate()
	if err != nil {
		return err
	}

	for i := range alerts {
		alert := &alerts[i]
		alert.Labels[partOfAlertLabelKey] = partOfAlertLabelValue
		alert.Labels[componentAlertLabelKey] = componentAlertLabelValue
		alert.Annotations[prometheusRunbookAnnotationKey] = fmt.Sprintf(runbookURLTemplate, alert.Alert)
	}

	return registry.RegisterAlerts(alerts)
}

func getRunbookURLTemplate() (string, error) {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		return "", errors.New("runbook URL template must have exactly 1 %s substring")
	}

	return runbookURLTemplate, nil
}

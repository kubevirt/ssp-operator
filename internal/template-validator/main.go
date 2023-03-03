package main

import (
	"os"

	"kubevirt.io/ssp-operator/internal/template-validator/service"
	"kubevirt.io/ssp-operator/internal/template-validator/validator"
)

func Main() int {
	app := &validator.App{}
	service.Setup(app)
	app.Run()
	return 0
}

func main() {
	os.Exit(Main())
}

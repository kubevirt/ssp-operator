package logger

import ctrl "sigs.k8s.io/controller-runtime"

var Log = ctrl.Log.WithName("template-validator")

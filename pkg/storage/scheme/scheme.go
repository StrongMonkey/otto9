package scheme

import (
	"github.com/otto8-ai/nah/pkg/restconfig"
	"github.com/otto8-ai/otto8/pkg/storage/apis/otto.otto8.ai/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
)

var (
	Scheme,
	Codecs,
	Parameter,
	AddToScheme = restconfig.MustBuildScheme(
		v1.AddToScheme,
		coordinationv1.AddToScheme,
		corev1.AddToScheme)
)

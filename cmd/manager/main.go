/*
Copyright 2020 VMware Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	// credential providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	certmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/thirdparty/cert-manager/v1"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/controllers"
	// +kubebuilder:scaffold:imports
)

const (
	cacheMountPath        = "/var/cache/ggcr"
	additionalCAMountPath = "/var/conventions/tls/ca-certificates.crt"
	metricsconfigMapName  = "controller-manager-metrics-data"
)

var (
	scheme     = runtime.NewScheme()
	setupLog   = ctrl.Log.WithName("setup")
	syncPeriod = 10 * time.Hour
	namespace  = os.Getenv("SYSTEM_NAMESPACE")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)
	_ = certmanagerv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	var metricsAddr string
	var probesAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&probesAddr, "probes-addr", ":8081", "The address health probes bind to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		TimeEncoder: zapcore.RFC3339NanoTimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probesAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "conventions-controller-leader-election-helper",
		Cache: ctrlcache.Options{
			SyncPeriod: &syncPeriod,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				// wokeignore:rule=disable
				DisableFor: []client.Object{
					&corev1.Secret{},
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "there was an error initializing a new `Manager` object")
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "there was an error creating a new clientset using config provided")
		os.Exit(1)
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver("")
	if err != nil {
		setupLog.Error(err, "there was an error creating a new authentication info resolver object")
		os.Exit(1)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: authInfoResolver,
		ServiceResolver:  webhookutil.NewDefaultServiceResolver(),
	}
	rc := binding.RegistryConfig{
		Cache:      cache.NewFilesystemCache(cacheMountPath),
		Client:     client,
		CACertPath: additionalCAMountPath,
	}
	// extension controllers

	if err = controllers.PodIntentReconciler(
		reconcilers.NewConfig(mgr, &conventionsv1alpha1.PodIntent{}, syncPeriod),
		wc,
		rc,
	).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodIntent")
		os.Exit(1)
	}
	if err = ctrl.NewWebhookManagedBy(mgr).For(&conventionsv1alpha1.PodIntent{}).Complete(); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "PodIntent")
		os.Exit(1)
	}

	if err = ctrl.NewWebhookManagedBy(mgr).For(&conventionsv1alpha1.ClusterPodConvention{}).Complete(); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterPodConvention")
		os.Exit(1)
	}

	setupLog.Info("starting metrics reconciler")
	if err = (&controllers.MetricsReconciler{
		Client:    mgr.GetClient(),
		Namespace: namespace,
		Name:      metricsconfigMapName,
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetricsReconciler")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("default", func(_ *http.Request) error { return nil }); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("default", func(_ *http.Request) error { return nil }); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "there was an error starting the manager")
		os.Exit(1)
	}
}

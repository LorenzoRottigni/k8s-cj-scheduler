package main

import (
	"flag"
	"net/http"
	"os"

	schedulingapiv1 "github.com/lorenzorottigni/k8s-cj-scheduler/api/v1"
	"github.com/lorenzorottigni/k8s-cj-scheduler/internal/controller"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server" // Import metrics server package
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(schedulingapiv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		Metrics:        metricsserver.Options{BindAddress: metricsAddr}, // Updated metrics configuration
		LeaderElection: enableLeaderElection,
		LeaderElectionID: "scheduler-controller.rottigni.tech",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.SchedulerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Scheduler")
		os.Exit(1)
	}

	// Start health and readiness HTTP server in background
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		})

		setupLog.Info("starting health server on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			setupLog.Error(err, "health server failed")
			os.Exit(1)
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

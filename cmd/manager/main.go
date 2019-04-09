/*

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
	"os"

	"github.com/anfernee/k8s-ipam-webhook/pkg/apis"
	ipamv1beta1 "github.com/anfernee/k8s-ipam-webhook/pkg/apis/ipam/v1beta1"
	"github.com/anfernee/k8s-ipam-webhook/pkg/webhook/ipam"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sig.k8s.io/cluster-api/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
)

func main() {
	disableWebhookConfigInstaller := false

	var metricsAddr string
	var host string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&host, "host", "localhost", "IP address to listen to")
	flag.Parse()

	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("entrypoint")

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: metricsAddr})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	log.Info("Setting up controller")
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "unable to register controllers to the manager")
		os.Exit(1)
	}

	// register webhook
	log.Info("setting up webhooks")
	mutatingWebhook, err := builder.NewWebhookBuilder().
		Name("mutating.k8s.io").
		Mutating().
		Operations(admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update).
		WithManager(mgr).
		ForType(&ipamv1beta1.Machine{}).
		Handlers(&ipam.IPAMAllocator{}).
		Build()
	if err != nil {
		log.Error(err, "unable to setup mutating webhook")
		os.Exit(1)
	}

	log.Info("setting up webhook server")
	as, err := webhook.NewServer("webhook-admission-server", mgr, webhook.ServerOptions{
		BootstrapOptions: &webhook.BootstrapOptions{
			Host: &host,
		},
		Port:                          9876,
		CertDir:                       "/tmp/cert",
		DisableWebhookConfigInstaller: &disableWebhookConfigInstaller,
	})
	if err != nil {
		log.Error(err, "unable to create a new webhook server")
		os.Exit(1)
	}

	err = as.Register(mutatingWebhook)
	if err != nil {
		log.Error(err, "unable to register webhooks in the admission server")
		os.Exit(1)
	}

	// Start the Cmd
	log.Info("Starting the Cmd.")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}

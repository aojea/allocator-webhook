/*
Copyright 2018 The Kubernetes Authors.
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

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	clusteripv1 "github.com/aojea/clusterip-webhook/api/v1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	log.SetLogger(zap.New())
	utilruntime.Must(clusteripv1.AddToScheme(scheme))

}

func main() {
	entryLog := log.Log.WithName("entrypoint")
	var serviceSubnet string
	flag.StringVar(&serviceSubnet, "service-subnet", "10.96.0.0/16", "The service subnet.")

	// Setup a Manager
	entryLog.Info("setting up manager")
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// Setup webhooks
	entryLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()
	svcAllocator, err := NewClusterIPAllocator(mgr.GetClient(), serviceSubnet)
	if err != nil {
		entryLog.Error(err, "unable to create webhook", "webhook", "ServiceAllocator")
		os.Exit(1)
	}
	hookServer.Register("/mutate-clusterip", &webhook.Admission{Handler: svcAllocator})

	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

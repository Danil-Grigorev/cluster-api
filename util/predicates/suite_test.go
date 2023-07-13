/*
Copyright 2021 The Kubernetes Authors.

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

package predicates

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/internal/test/envtest"
)

var (
	ctx     = ctrl.SetupSignalHandler()
	timeout = 10 * time.Second
	env     *envtest.Environment
)

// Reconciler reconciles a Machine object.
type Reconciler struct {
	Client               client.Client
	WatchFilterPredicate LabelMatcher
}

func (r *Reconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Machine{}).
		WithEventFilter(r.WatchFilterPredicate.Matches(logger)).
		WithOptions(opts).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	m := &clusterv1.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, m); err != nil {
		return ctrl.Result{}, err
	}

	patch := client.MergeFrom(m.DeepCopy())
	m.Status.BootstrapReady = true

	return ctrl.Result{}, r.Client.Status().Patch(ctx, m, patch)
}

func TestMain(m *testing.M) {
	matcher, err := InitLabelMatcher(logger, "!some,one,cluster.x-k8s.io/watch-filter = value")
	if err != nil {
		panic(fmt.Sprintf("unable to setup matcher expression: %v", err))
	}
	setupReconcilers := func(ctx context.Context, mgr ctrl.Manager) {
		if err := (&Reconciler{
			Client:               mgr.GetClient(),
			WatchFilterPredicate: matcher,
		}).SetupWithManager(ctx, mgr, controller.Options{}); err != nil {
			panic(fmt.Sprintf("unable to create machine reconciler: %v", err))
		}
	}

	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		CacheOptions: cache.Options{
			DefaultLabelSelector: matcher.selector,
		},
		SetupEnv:         func(e *envtest.Environment) { env = e },
		SetupReconcilers: setupReconcilers,
	}))
}
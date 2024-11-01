/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/myporject/test/api/v1alpha1"
)

var logger = log.Log.WithName("test_controller")

// TestReconciler reconciles a Test object
type TestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.kylian.test.com,resources=tests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.kylian.test.com,resources=tests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.kylian.test.com,resources=tests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Test object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *TestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	logger.Info("Reconcile called")

	//crate Test resource
	test := &apiv1alpha1.Test{}
	err := r.Get(ctx, req.NamespacedName, test)
	if err != nil {
		// ignore not found error
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	startTime := test.Spec.StartTime
	endTime := test.Spec.EndTime
	replicaes := test.Spec.Replicas

	currenHour := time.Now().Local().Hour()
	logger.Info(fmt.Sprintf("Current hour: %d", currenHour))

	if currenHour >= startTime && currenHour <= endTime {
		logger.Info("Start to call testdepployment")
		err := testdepployment(test, r, ctx, *replicaes)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func testdepployment(test *apiv1alpha1.Test, r *TestReconciler, ctx context.Context, replicas int32) error {

	// create deployment
	for _, deploy := range test.Spec.Deployments {
		deploymentnew := &v1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, deploymentnew)
		if err != nil {
			return err
		}

		// check and update replicas
		if deploymentnew.Spec.Replicas != &replicas {
			deploymentnew.Spec.Replicas = &replicas
			err := r.Update(ctx, deploymentnew)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Test{}).
		Complete(r)
}

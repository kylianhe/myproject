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
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/myporject/test/api/v1alpha1"
)

const finalizer = "test.kylian.test.com/finalizer"

var logger = log.Log.WithName("test_controller")

var deploymentInfo = make(map[string]apiv1alpha1.DeploymentInfo)
var annotation = make(map[string]string)

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

	if test.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(test, finalizer) {
			controllerutil.AddFinalizer(test, finalizer)
			logger.Info("Add finalizer", finalizer)
			err := r.Update(ctx, test)
			if err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}

		if test.Status.Status == "" {
			test.Status.Status = apiv1alpha1.Pending
			err := r.Status().Update(ctx, test)
			if err != nil {
				return ctrl.Result{}, err
			}

			if err := addAnnotations(test, r, ctx); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

		}

		startTime := test.Spec.StartTime
		endTime := test.Spec.EndTime
		replicaes := test.Spec.Replicas

		currenHour := time.Now().Local().Hour()
		logger.Info(fmt.Sprintf("Current hour: %d", currenHour))

		if currenHour >= startTime && currenHour <= endTime {
			if test.Status.Status != apiv1alpha1.Success {
				logger.Info("Start to call testdepployment")
				err := testdepployment(test, r, ctx, replicaes)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		} else {
			if test.Status.Status == apiv1alpha1.Success {
				err := restoredeployment(test, r, ctx)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	} else {
		logger.Info("Deleting test resource")
		if test.Status.Status == apiv1alpha1.Success {
			err := restoredeployment(test, r, ctx)
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("Remove finalizer")
			controllerutil.RemoveFinalizer(test, finalizer)
		}

		logger.Info("Delete test resource")
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func restoredeployment(test *apiv1alpha1.Test, r *TestReconciler, ctx context.Context) error {

	logger.Info("Restore deployment")
	for name, deployInfo := range deploymentInfo {
		deploymentnew := &v1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: deployInfo.Namespace, Name: name}, deploymentnew); err != nil {
			return err
		}

		if deploymentnew.Spec.Replicas != &deployInfo.Replicas {
			logger.Info("Restore deployment replicas")
			deploymentnew.Spec.Replicas = &deployInfo.Replicas
			err := r.Update(ctx, deploymentnew)
			if err != nil {
				return err
			}
		}
	}

	test.Status.Status = apiv1alpha1.Restore
	err := r.Status().Update(ctx, test)
	if err != nil {

	}

	return nil
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
				test.Status.Status = apiv1alpha1.Failed
				err := r.Status().Update(ctx, test)
				if err != nil {
					return err
				}
				return err
			}

			test.Status.Status = apiv1alpha1.Success
			err = r.Status().Update(ctx, test)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func addAnnotations(test *apiv1alpha1.Test, r *TestReconciler, ctx context.Context) error {

	// get deploymentinfo and add annotations
	for _, deploy := range test.Spec.Deployments {
		deploymentnew := &v1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, deploymentnew); err != nil {
			return err
		}

		// add annotations to deployment
		if *deploymentnew.Spec.Replicas != test.Spec.Replicas {
			logger.Info("Add annotations to deployment")
			deploymentInfo[deploymentnew.Name] = apiv1alpha1.DeploymentInfo{
				Namespace: deploymentnew.Namespace,
				Replicas:  *deploymentnew.Spec.Replicas,
			}
		}
	}

	for deploymentName, info := range deploymentInfo {

		infoJson, err := json.Marshal(info)
		if err != nil {
			return err
		}

		// add annotations to deployment
		annotation[deploymentName] = string(infoJson)
	}

	// update annotations to test resource
	test.ObjectMeta.Annotations = annotation
	err := r.Update(ctx, test)
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Test{}).
		Complete(r)
}

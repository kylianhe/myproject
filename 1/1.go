package 1

import "k8s.io/apimachinery/pkg/types"

func addAnnotations(test *apiv1alpha1.Test, r *TestReconciler, ctx context.Context) error {

	// get deploymentinfo from deployment and save it in originalDeploymentInfo
	for _, deploy := range test.Spec.Deployments {
		doploymentnew := &v1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, doploymentnew); err != nil {
			return err
		}

		if *doploymentnew.Spec.Replicas != test.Spec.Replicas {
			logger.Info("Deployment replicas not equal to test replicas")
			deploymentInfo[deploymentnew.Name] = apiv1alpha1.DeploymentInfo{
				Replicas:  *doploymentnew.Spec.Replicas,
				Namespace: deploymentnew.Namespace,
			}
		}
	}

	return nil
}

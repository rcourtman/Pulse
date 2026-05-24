package kubernetesagent

import (
	"context"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCollectNativePolicyConfigAndAutoscalingInventory(t *testing.T) {
	immutable := true
	minAvailable := intstr.FromInt(1)
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{UID: "secret-uid", Namespace: "apps", Name: "api-secret"},
			Type:       corev1.SecretTypeOpaque,
			Immutable:  &immutable,
			Data:       map[string][]byte{"token": []byte("super-secret")},
		},
		&corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{UID: "quota-uid", Namespace: "apps", Name: "apps-quota"},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{corev1.ResourcePods: k8sresource.MustParse("10")},
			},
			Status: corev1.ResourceQuotaStatus{
				Hard: corev1.ResourceList{corev1.ResourcePods: k8sresource.MustParse("10")},
				Used: corev1.ResourceList{corev1.ResourcePods: k8sresource.MustParse("3")},
			},
		},
		&corev1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{UID: "limits-uid", Namespace: "apps", Name: "apps-limits"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{Type: corev1.LimitTypeContainer}},
			},
		},
		&policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{UID: "pdb-uid", Namespace: "apps", Name: "api-pdb"},
			Spec:       policyv1.PodDisruptionBudgetSpec{MinAvailable: &minAvailable},
			Status: policyv1.PodDisruptionBudgetStatus{
				DesiredHealthy:     1,
				CurrentHealthy:     1,
				DisruptionsAllowed: 1,
				ExpectedPods:       2,
			},
		},
		&autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{UID: "hpa-uid", Namespace: "apps", Name: "api-hpa"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "api"},
				MinReplicas:    int32Ptr(2),
				MaxReplicas:    10,
				Metrics: []autoscalingv2.MetricSpec{{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: int32Ptr(70),
						},
					},
				}},
			},
			Status: autoscalingv2.HorizontalPodAutoscalerStatus{CurrentReplicas: 2, DesiredReplicas: 3},
		},
	)
	a := &Agent{kubeClient: clientset}

	secrets, err := a.collectSecrets(context.Background())
	if err != nil {
		t.Fatalf("collectSecrets: %v", err)
	}
	if len(secrets) != 1 || secrets[0].Type != string(corev1.SecretTypeOpaque) || len(secrets[0].DataKeys) != 1 || secrets[0].DataKeys[0] != "token" || !secrets[0].Immutable {
		t.Fatalf("unexpected secrets: %+v", secrets)
	}

	quotas, err := a.collectResourceQuotas(context.Background())
	if err != nil {
		t.Fatalf("collectResourceQuotas: %v", err)
	}
	if len(quotas) != 1 || quotas[0].Hard["pods"] != "10" || quotas[0].Used["pods"] != "3" {
		t.Fatalf("unexpected quotas: %+v", quotas)
	}

	limitRanges, err := a.collectLimitRanges(context.Background())
	if err != nil {
		t.Fatalf("collectLimitRanges: %v", err)
	}
	if len(limitRanges) != 1 || len(limitRanges[0].LimitTypes) != 1 || limitRanges[0].LimitTypes[0] != "Container" {
		t.Fatalf("unexpected limit ranges: %+v", limitRanges)
	}

	budgets, err := a.collectPodDisruptionBudgets(context.Background())
	if err != nil {
		t.Fatalf("collectPodDisruptionBudgets: %v", err)
	}
	if len(budgets) != 1 || budgets[0].MinAvailable != "1" || budgets[0].ExpectedPods != 2 || budgets[0].DisruptionsAllowed != 1 {
		t.Fatalf("unexpected pod disruption budgets: %+v", budgets)
	}

	autoscalers, err := a.collectHorizontalPodAutoscalers(context.Background())
	if err != nil {
		t.Fatalf("collectHorizontalPodAutoscalers: %v", err)
	}
	if len(autoscalers) != 1 || autoscalers[0].TargetName != "api" || autoscalers[0].MinReplicas != 2 || autoscalers[0].MaxReplicas != 10 || autoscalers[0].MetricTypes[0] != "Resource:cpu" {
		t.Fatalf("unexpected autoscalers: %+v", autoscalers)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

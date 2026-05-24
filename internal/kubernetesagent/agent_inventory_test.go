package kubernetesagent

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCollectNativePolicyConfigAndAutoscalingInventory(t *testing.T) {
	minAvailable := intstr.FromInt(1)
	clientset := fake.NewSimpleClientset(
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
	clientset.PrependReactor("list", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("full configmap payload list should not be called")
	})
	clientset.PrependReactor("list", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("full secret payload list should not be called")
	})

	metadataScheme := metadatafake.NewTestScheme()
	metav1.AddMetaToScheme(metadataScheme)
	metadataClient := metadatafake.NewSimpleMetadataClient(
		metadataScheme,
		&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				UID:               "configmap-uid",
				Namespace:         "apps",
				Name:              "api-config",
				CreationTimestamp: metav1.Now(),
				Labels:            map[string]string{"app": "api"},
			},
		},
		&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				UID:               "secret-uid",
				Namespace:         "apps",
				Name:              "api-secret",
				CreationTimestamp: metav1.Now(),
				Labels:            map[string]string{"app": "api"},
			},
		},
	)
	a := &Agent{kubeClient: clientset, metadataClient: metadataClient}

	configMaps, err := a.collectConfigMaps(context.Background())
	if err != nil {
		t.Fatalf("collectConfigMaps: %v", err)
	}
	if len(configMaps) != 1 || configMaps[0].Name != "api-config" || !configMaps[0].MetadataOnly || len(configMaps[0].DataKeys) != 0 || len(configMaps[0].BinaryDataKeys) != 0 {
		t.Fatalf("unexpected configmaps: %+v", configMaps)
	}

	secrets, err := a.collectSecrets(context.Background())
	if err != nil {
		t.Fatalf("collectSecrets: %v", err)
	}
	if len(secrets) != 1 || secrets[0].Name != "api-secret" || !secrets[0].MetadataOnly || secrets[0].Type != "" || len(secrets[0].DataKeys) != 0 || secrets[0].Immutable {
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

func TestCollectDeploymentInventoryPreservesAPIMetadata(t *testing.T) {
	replicas := int32(4)
	createdAt := metav1.NewTime(time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC))
	clientset := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			UID:               "deployment-uid-1",
			Namespace:         "services",
			Name:              "checkout-api",
			CreationTimestamp: createdAt,
			Labels:            map[string]string{"app": "checkout"},
		},
		Spec: appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 12,
			UpdatedReplicas:    3,
			ReadyReplicas:      2,
			AvailableReplicas:  2,
		},
	})
	a := &Agent{kubeClient: clientset}

	deployments, err := a.collectDeployments(context.Background())
	if err != nil {
		t.Fatalf("collectDeployments: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected one deployment, got %+v", deployments)
	}

	deployment := deployments[0]
	if deployment.UID != "deployment-uid-1" || deployment.Name != "checkout-api" || deployment.Namespace != "services" {
		t.Fatalf("deployment identity metadata not preserved: %+v", deployment)
	}
	if !deployment.CreatedAt.Equal(createdAt.Time) || deployment.ObservedGeneration != 12 {
		t.Fatalf("deployment API metadata not preserved: %+v", deployment)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

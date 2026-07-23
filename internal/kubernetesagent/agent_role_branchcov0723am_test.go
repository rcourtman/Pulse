package kubernetesagent

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestBranchcov0723Am_KubernetesTargetRole covers every arm of
// kubernetesTargetRole. The source implementation only inspects the
// TargetConfig.Authoritative boolean (there is no Role string field, so
// case/whitespace/unknown-role branches described in some specs simply do not
// exist in the code).
func TestBranchcov0723Am_KubernetesTargetRole(t *testing.T) {
	t.Run("zero_value_returns_observer", func(t *testing.T) {
		// Zero-value TargetConfig has Authoritative == false.
		if got := kubernetesTargetRole(TargetConfig{}); got != "observer" {
			t.Fatalf("kubernetesTargetRole(zero) = %q, want %q", got, "observer")
		}
	})

	t.Run("authoritative_returns_primary", func(t *testing.T) {
		cfg := TargetConfig{Authoritative: true}
		if got := kubernetesTargetRole(cfg); got != "primary" {
			t.Fatalf("kubernetesTargetRole(Authoritative) = %q, want %q", got, "primary")
		}
	})
}

func TestBranchcov0723Am_TimePtrFromMeta(t *testing.T) {
	t.Run("zero_time_returns_nil", func(t *testing.T) {
		if got := timePtrFromMeta(metav1.Time{}); got != nil {
			t.Fatalf("timePtrFromMeta(zero) = %v, want nil", *got)
		}
	})

	t.Run("non_zero_time_returns_pointer_copy", func(t *testing.T) {
		ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		got := timePtrFromMeta(metav1.Time{Time: ts})
		if got == nil {
			t.Fatal("expected non-nil pointer")
		}
		if !got.Equal(ts) {
			t.Fatalf("timePtrFromMeta = %v, want %v", *got, ts)
		}
		// Returned pointer must be a copy, not alias the input's field.
		mt := metav1.Time{Time: ts}
		p := timePtrFromMeta(mt)
		mt.Time = mt.Time.Add(time.Hour)
		if !p.Equal(ts) {
			t.Fatalf("timePtrFromMeta aliased input: now %v, want %v", *p, ts)
		}
	})
}

func TestBranchcov0723Am_TimePtrFromMicro(t *testing.T) {
	t.Run("zero_time_returns_nil", func(t *testing.T) {
		if got := timePtrFromMicro(metav1.MicroTime{}); got != nil {
			t.Fatalf("timePtrFromMicro(zero) = %v, want nil", *got)
		}
	})

	t.Run("non_zero_time_returns_pointer_copy", func(t *testing.T) {
		ts := time.Date(2024, 5, 6, 7, 8, 9, 123456, time.UTC)
		got := timePtrFromMicro(metav1.MicroTime{Time: ts})
		if got == nil {
			t.Fatal("expected non-nil pointer")
		}
		if !got.Equal(ts) {
			t.Fatalf("timePtrFromMicro = %v, want %v", *got, ts)
		}
	})
}

func TestBranchcov0723Am_TimePtrFromMetaPtr(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := timePtrFromMetaPtr(nil); got != nil {
			t.Fatalf("timePtrFromMetaPtr(nil) = %v, want nil", *got)
		}
	})

	t.Run("nil_safe_zero_time_returns_nil", func(t *testing.T) {
		mt := metav1.Time{}
		if got := timePtrFromMetaPtr(&mt); got != nil {
			t.Fatalf("timePtrFromMetaPtr(&zero) = %v, want nil", *got)
		}
	})

	t.Run("non_zero_time_returns_pointer", func(t *testing.T) {
		ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		mt := metav1.Time{Time: ts}
		got := timePtrFromMetaPtr(&mt)
		if got == nil {
			t.Fatal("expected non-nil pointer")
		}
		if !got.Equal(ts) {
			t.Fatalf("timePtrFromMetaPtr = %v, want %v", *got, ts)
		}
	})
}

func TestBranchcov0723Am_AccessModes(t *testing.T) {
	t.Run("empty_returns_nil", func(t *testing.T) {
		if got := accessModes(nil); got != nil {
			t.Fatalf("accessModes(nil) = %v, want nil", got)
		}
		if got := accessModes([]corev1.PersistentVolumeAccessMode{}); got != nil {
			t.Fatalf("accessModes([]) = %v, want nil", got)
		}
	})

	t.Run("whitespace_only_entries_filtered", func(t *testing.T) {
		got := accessModes([]corev1.PersistentVolumeAccessMode{"  ", ""})
		if len(got) != 0 {
			t.Fatalf("accessModes(whitespace) = %v, want empty", got)
		}
	})

	t.Run("modes_returned_trimmed", func(t *testing.T) {
		got := accessModes([]corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
			corev1.ReadOnlyMany,
			corev1.ReadWriteMany,
		})
		want := []string{
			string(corev1.ReadWriteOnce),
			string(corev1.ReadOnlyMany),
			string(corev1.ReadWriteMany),
		}
		if len(got) != len(want) {
			t.Fatalf("accessModes len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("accessModes[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestBranchcov0723Am_StorageClassNamePtr(t *testing.T) {
	t.Run("nil_returns_empty", func(t *testing.T) {
		if got := storageClassNamePtr(nil); got != "" {
			t.Fatalf("storageClassNamePtr(nil) = %q, want empty", got)
		}
	})

	t.Run("value_trimmed", func(t *testing.T) {
		name := "  fast-ssd  "
		if got := storageClassNamePtr(&name); got != "fast-ssd" {
			t.Fatalf("storageClassNamePtr = %q, want %q", got, "fast-ssd")
		}
	})

	t.Run("plain_value_passes_through", func(t *testing.T) {
		name := "standard"
		if got := storageClassNamePtr(&name); got != "standard" {
			t.Fatalf("storageClassNamePtr = %q, want %q", got, "standard")
		}
	})
}

func TestBranchcov0723Am_BoolPtrCopy(t *testing.T) {
	t.Run("nil_returns_nil", func(t *testing.T) {
		if got := boolPtrCopy(nil); got != nil {
			t.Fatalf("boolPtrCopy(nil) = %v, want nil", *got)
		}
	})

	t.Run("true_copied", func(t *testing.T) {
		v := true
		got := boolPtrCopy(&v)
		if got == nil || *got != true {
			t.Fatalf("boolPtrCopy(true) = %v, want true", got)
		}
		// Mutating input must not affect the returned copy.
		v = false
		if *got != true {
			t.Fatal("boolPtrCopy aliased input value")
		}
	})

	t.Run("false_copied", func(t *testing.T) {
		v := false
		got := boolPtrCopy(&v)
		if got == nil || *got != false {
			t.Fatalf("boolPtrCopy(false) = %v, want false", got)
		}
	})
}

func TestBranchcov0723Am_StringPtrValue(t *testing.T) {
	t.Run("nil_returns_empty", func(t *testing.T) {
		if got := stringPtrValue(nil); got != "" {
			t.Fatalf("stringPtrValue(nil) = %q, want empty", got)
		}
	})

	t.Run("value_trimmed", func(t *testing.T) {
		v := "  hello  "
		if got := stringPtrValue(&v); got != "hello" {
			t.Fatalf("stringPtrValue = %q, want %q", got, "hello")
		}
	})
}

func TestBranchcov0723Am_SortedStringKeys(t *testing.T) {
	t.Run("empty_returns_nil", func(t *testing.T) {
		if got := sortedStringKeys[int](nil); got != nil {
			t.Fatalf("sortedStringKeys(nil) = %v, want nil", got)
		}
		if got := sortedStringKeys[int](map[string]int{}); got != nil {
			t.Fatalf("sortedStringKeys(empty) = %v, want nil", got)
		}
	})

	t.Run("whitespace_only_keys_filtered", func(t *testing.T) {
		got := sortedStringKeys[int](map[string]int{"   ": 1, "": 2})
		if len(got) != 0 {
			t.Fatalf("sortedStringKeys(whitespace) = %v, want empty", got)
		}
	})

	t.Run("keys_sorted_and_trimmed", func(t *testing.T) {
		got := sortedStringKeys[int](map[string]int{
			"  banana ": 1,
			"apple":     2,
			"cherry":    3,
		})
		want := []string{"apple", "banana", "cherry"}
		if len(got) != len(want) {
			t.Fatalf("sortedStringKeys len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("sortedStringKeys[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestBranchcov0723Am_DesiredReplicasAndCompletions(t *testing.T) {
	t.Run("statefulset_nil_replicas_zero", func(t *testing.T) {
		if got := statefulSetDesiredReplicas(appsv1.StatefulSet{}); got != 0 {
			t.Fatalf("statefulSetDesiredReplicas(nil) = %d, want 0", got)
		}
	})

	t.Run("statefulset_replicas_value", func(t *testing.T) {
		replicas := int32(5)
		ss := appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: &replicas}}
		if got := statefulSetDesiredReplicas(ss); got != 5 {
			t.Fatalf("statefulSetDesiredReplicas = %d, want 5", got)
		}
	})

	t.Run("replicaset_nil_replicas_zero", func(t *testing.T) {
		if got := replicaSetDesiredReplicas(appsv1.ReplicaSet{}); got != 0 {
			t.Fatalf("replicaSetDesiredReplicas(nil) = %d, want 0", got)
		}
	})

	t.Run("replicaset_replicas_value", func(t *testing.T) {
		replicas := int32(7)
		rs := appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{Replicas: &replicas}}
		if got := replicaSetDesiredReplicas(rs); got != 7 {
			t.Fatalf("replicaSetDesiredReplicas = %d, want 7", got)
		}
	})

	t.Run("job_nil_completions_zero", func(t *testing.T) {
		if got := jobDesiredCompletions(batchv1.Job{}); got != 0 {
			t.Fatalf("jobDesiredCompletions(nil) = %d, want 0", got)
		}
	})

	t.Run("job_completions_value", func(t *testing.T) {
		c := int32(3)
		j := batchv1.Job{Spec: batchv1.JobSpec{Completions: &c}}
		if got := jobDesiredCompletions(j); got != 3 {
			t.Fatalf("jobDesiredCompletions = %d, want 3", got)
		}
	})
}

func TestBranchcov0723Am_CronJobSuspended(t *testing.T) {
	t.Run("nil_returns_false", func(t *testing.T) {
		if got := cronJobSuspended(batchv1.CronJob{}); got {
			t.Fatal("cronJobSuspended(nil) = true, want false")
		}
	})

	t.Run("explicit_false_returns_false", func(t *testing.T) {
		f := false
		if got := cronJobSuspended(batchv1.CronJob{Spec: batchv1.CronJobSpec{Suspend: &f}}); got {
			t.Fatal("cronJobSuspended(false) = true, want false")
		}
	})

	t.Run("explicit_true_returns_true", func(t *testing.T) {
		tr := true
		if got := cronJobSuspended(batchv1.CronJob{Spec: batchv1.CronJobSpec{Suspend: &tr}}); !got {
			t.Fatal("cronJobSuspended(true) = false, want true")
		}
	})
}

func TestBranchcov0723Am_IngressClassName(t *testing.T) {
	t.Run("nil_returns_empty", func(t *testing.T) {
		if got := ingressClassName(networkingv1.Ingress{}); got != "" {
			t.Fatalf("ingressClassName(nil) = %q, want empty", got)
		}
	})

	t.Run("value_trimmed", func(t *testing.T) {
		name := "  nginx  "
		ing := networkingv1.Ingress{Spec: networkingv1.IngressSpec{IngressClassName: &name}}
		if got := ingressClassName(ing); got != "nginx" {
			t.Fatalf("ingressClassName = %q, want %q", got, "nginx")
		}
	})
}

func TestBranchcov0723Am_IngressHosts(t *testing.T) {
	t.Run("empty_rules_returns_empty_nonnil", func(t *testing.T) {
		got := ingressHosts(networkingv1.Ingress{})
		if got == nil {
			t.Fatalf("ingressHosts(empty) = nil, want a non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("ingressHosts(empty) = %v, want empty", got)
		}
	})

	t.Run("blank_hosts_filtered", func(t *testing.T) {
		ing := networkingv1.Ingress{Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{
			{Host: "   "},
			{Host: ""},
		}}}
		got := ingressHosts(ing)
		if len(got) != 0 {
			t.Fatalf("ingressHosts(blank) = %v, want empty", got)
		}
	})

	t.Run("hosts_trimmed_deduped_order_preserved", func(t *testing.T) {
		ing := networkingv1.Ingress{Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{
			{Host: "  b.example.com  "},
			{Host: "a.example.com"},
			{Host: "b.example.com"}, // duplicate (after trim)
			{Host: "c.example.com"},
		}}}
		got := ingressHosts(ing)
		want := []string{"b.example.com", "a.example.com", "c.example.com"}
		if len(got) != len(want) {
			t.Fatalf("ingressHosts len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ingressHosts[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestBranchcov0723Am_IngressAddresses(t *testing.T) {
	t.Run("empty_status_returns_empty_nonnil", func(t *testing.T) {
		got := ingressAddresses(networkingv1.Ingress{})
		if len(got) != 0 {
			t.Fatalf("ingressAddresses(empty) = %v, want empty", got)
		}
	})

	t.Run("ips_and_hostnames_deduped_trimmed", func(t *testing.T) {
		ing := networkingv1.Ingress{Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
			Ingress: []networkingv1.IngressLoadBalancerIngress{
				{IP: "  10.0.0.1  ", Hostname: ""},
				{IP: "10.0.0.1"}, // duplicate IP after trim
				{Hostname: "  lb.example.com  "},
				{IP: "10.0.0.2"},
			},
		}}}
		got := ingressAddresses(ing)
		want := []string{"10.0.0.1", "lb.example.com", "10.0.0.2"}
		if len(got) != len(want) {
			t.Fatalf("ingressAddresses len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ingressAddresses[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestBranchcov0723Am_NetworkPolicyTypes(t *testing.T) {
	t.Run("empty_returns_nil", func(t *testing.T) {
		if got := networkPolicyTypes(networkingv1.NetworkPolicy{}); got != nil {
			t.Fatalf("networkPolicyTypes(empty) = %v, want nil", got)
		}
	})

	t.Run("types_returned_trimmed_blank_filtered", func(t *testing.T) {
		policy := networkingv1.NetworkPolicy{Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyType("   "),
			},
		}}
		got := networkPolicyTypes(policy)
		want := []string{
			string(networkingv1.PolicyTypeIngress),
			string(networkingv1.PolicyTypeEgress),
		}
		if len(got) != len(want) {
			t.Fatalf("networkPolicyTypes len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("networkPolicyTypes[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestBranchcov0723Am_EndpointSliceServiceName(t *testing.T) {
	t.Run("nil_labels_returns_empty", func(t *testing.T) {
		if got := endpointSliceServiceName(discoveryv1.EndpointSlice{}); got != "" {
			t.Fatalf("endpointSliceServiceName(nil labels) = %q, want empty", got)
		}
	})

	t.Run("missing_label_returns_empty", func(t *testing.T) {
		slice := discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"other": "value"}}}
		if got := endpointSliceServiceName(slice); got != "" {
			t.Fatalf("endpointSliceServiceName(missing) = %q, want empty", got)
		}
	})

	t.Run("label_value_trimmed", func(t *testing.T) {
		slice := discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
			discoveryv1.LabelServiceName: "  my-service  ",
		}}}
		if got := endpointSliceServiceName(slice); got != "my-service" {
			t.Fatalf("endpointSliceServiceName = %q, want %q", got, "my-service")
		}
	})
}

func TestBranchcov0723Am_EndpointSlicePorts(t *testing.T) {
	t.Run("empty_returns_nil", func(t *testing.T) {
		if got := endpointSlicePorts(discoveryv1.EndpointSlice{}); got != nil {
			t.Fatalf("endpointSlicePorts(empty) = %v, want nil", got)
		}
	})

	t.Run("nil_port_and_protocol_default_to_zero_and_empty", func(t *testing.T) {
		name := "http"
		appProto := "  L7  "
		slice := discoveryv1.EndpointSlice{Ports: []discoveryv1.EndpointPort{
			{Name: &name, AppProtocol: &appProto},
		}}
		got := endpointSlicePorts(slice)
		if len(got) != 1 {
			t.Fatalf("endpointSlicePorts len = %d, want 1", len(got))
		}
		if got[0].Port != 0 {
			t.Fatalf("Port = %d, want 0", got[0].Port)
		}
		if got[0].Protocol != "" {
			t.Fatalf("Protocol = %q, want empty", got[0].Protocol)
		}
		if got[0].Name != "http" {
			t.Fatalf("Name = %q, want http", got[0].Name)
		}
		if got[0].AppProtocol != "L7" {
			t.Fatalf("AppProtocol = %q, want L7", got[0].AppProtocol)
		}
	})

	t.Run("set_port_and_protocol_returned", func(t *testing.T) {
		port := int32(443)
		proto := corev1.ProtocolTCP
		name := "https"
		slice := discoveryv1.EndpointSlice{Ports: []discoveryv1.EndpointPort{
			{Port: &port, Protocol: &proto, Name: &name},
		}}
		got := endpointSlicePorts(slice)
		if len(got) != 1 {
			t.Fatalf("endpointSlicePorts len = %d, want 1", len(got))
		}
		if got[0].Port != 443 {
			t.Fatalf("Port = %d, want 443", got[0].Port)
		}
		if got[0].Protocol != string(corev1.ProtocolTCP) {
			t.Fatalf("Protocol = %q, want %q", got[0].Protocol, corev1.ProtocolTCP)
		}
		if got[0].Name != "https" {
			t.Fatalf("Name = %q, want https", got[0].Name)
		}
	})
}

func TestBranchcov0723Am_EndpointSliceReadyCount(t *testing.T) {
	t.Run("empty_endpoints_zero", func(t *testing.T) {
		if got := endpointSliceReadyCount(discoveryv1.EndpointSlice{}); got != 0 {
			t.Fatalf("endpointSliceReadyCount(empty) = %d, want 0", got)
		}
	})

	t.Run("nil_ready_counts_as_ready", func(t *testing.T) {
		slice := discoveryv1.EndpointSlice{Endpoints: []discoveryv1.Endpoint{
			{Conditions: discoveryv1.EndpointConditions{Ready: nil}},
			{Conditions: discoveryv1.EndpointConditions{Ready: nil}},
		}}
		if got := endpointSliceReadyCount(slice); got != 2 {
			t.Fatalf("endpointSliceReadyCount(nil) = %d, want 2", got)
		}
	})

	t.Run("mixed_ready_states_counted", func(t *testing.T) {
		tr := true
		fa := false
		slice := discoveryv1.EndpointSlice{Endpoints: []discoveryv1.Endpoint{
			{Conditions: discoveryv1.EndpointConditions{Ready: &tr}},
			{Conditions: discoveryv1.EndpointConditions{Ready: &fa}},
			{Conditions: discoveryv1.EndpointConditions{Ready: &tr}},
		}}
		if got := endpointSliceReadyCount(slice); got != 2 {
			t.Fatalf("endpointSliceReadyCount(mixed) = %d, want 2", got)
		}
	})
}

func TestBranchcov0723Am_ServiceAccountImagePullSecrets(t *testing.T) {
	t.Run("empty_returns_nil", func(t *testing.T) {
		if got := serviceAccountImagePullSecrets(corev1.ServiceAccount{}); got != nil {
			t.Fatalf("serviceAccountImagePullSecrets(empty) = %v, want nil", got)
		}
	})

	t.Run("secrets_trimmed_sorted_blank_filtered_no_dedup", func(t *testing.T) {
		sa := corev1.ServiceAccount{ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "  regcred  "},
			{Name: "regcred"}, // duplicate after trim; source does NOT dedupe
			{Name: "   "},     // blank, filtered
			{Name: "other"},
		}}
		got := serviceAccountImagePullSecrets(sa)
		// Source sorts the result; duplicates are preserved.
		want := []string{"other", "regcred", "regcred"}
		if len(got) != len(want) {
			t.Fatalf("serviceAccountImagePullSecrets len = %d, want %d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("serviceAccountImagePullSecrets[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

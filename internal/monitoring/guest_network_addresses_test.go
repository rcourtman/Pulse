package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestGuestAddressOrderingDoesNotAddFilteredOrMutateAddresses(t *testing.T) {
	t.Parallel()
	addresses := []string{"10.88.0.1", "192.0.2.10", "192.0.2.2", "unparsed-status"}
	interfaces := []models.GuestNetworkInterface{
		{Name: "eth0", Addresses: []string{"192.0.2.10", "fe80::1", "192.0.2.2"}},
		{Name: "podman0", Addresses: []string{"10.88.0.1"}},
	}
	beforeAddresses := cloneStringSlice(addresses)
	beforeInterfaces := cloneGuestNetworkInterfaces(interfaces)
	got := guestIPAddressesByInterface(addresses, interfaces)
	want := []string{"192.0.2.2", "192.0.2.10", "unparsed-status", "10.88.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(addresses, beforeAddresses) || !reflect.DeepEqual(interfaces, beforeInterfaces) {
		t.Fatal("address ordering mutated its input")
	}
}

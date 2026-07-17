package inventoryclient

import (
	"reflect"
	"testing"
)

func TestKitexClientExposesRuntimeState(t *testing.T) {
	if _, ok := reflect.TypeOf((*KitexClient)(nil)).MethodByName("GetRuntimeState"); !ok {
		t.Fatal("KitexClient must expose GetRuntimeState for readiness checks")
	}
}

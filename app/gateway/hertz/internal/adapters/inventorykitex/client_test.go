package inventorykitex

import (
	"reflect"
	"testing"
)

func TestClientExposesRuntimeState(t *testing.T) {
	if _, ok := reflect.TypeOf((*Client)(nil)).MethodByName("GetRuntimeState"); !ok {
		t.Fatal("Client must expose GetRuntimeState for readiness checks")
	}
}

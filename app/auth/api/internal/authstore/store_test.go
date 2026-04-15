package authstore

import "testing"

func TestStoreCreateSessionForDevice_SameDeviceReplacesOldSession(t *testing.T) {
	store := NewStore("pwd")

	_, firstRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	_, secondRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	if _, _, err := store.RefreshSession(firstRefresh, 3600); err == nil {
		t.Fatalf("expected first same-device session to be invalidated")
	}
	if _, _, err := store.RefreshSession(secondRefresh, 3600); err != nil {
		t.Fatalf("expected latest same-device session to stay active: %v", err)
	}
}

func TestStoreCreateSessionForDevice_DifferentDevicesCanCoexist(t *testing.T) {
	store := NewStore("pwd")

	_, webRefresh, err := store.CreateSessionForDevice(1001, "web", 3600)
	if err != nil {
		t.Fatalf("create web session: %v", err)
	}
	_, iosRefresh, err := store.CreateSessionForDevice(1001, "ios", 3600)
	if err != nil {
		t.Fatalf("create ios session: %v", err)
	}

	if _, _, err := store.RefreshSession(webRefresh, 3600); err != nil {
		t.Fatalf("expected web session to remain active: %v", err)
	}
	if _, _, err := store.RefreshSession(iosRefresh, 3600); err != nil {
		t.Fatalf("expected ios session to remain active: %v", err)
	}
}

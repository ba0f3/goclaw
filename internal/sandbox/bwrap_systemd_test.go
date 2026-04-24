package sandbox

import (
	"reflect"
	"testing"
)

func TestSystemdScopeArgsForEUID(t *testing.T) {
	tests := []struct {
		name string
		euid int
		want []string
	}{
		{name: "root", euid: 0, want: []string{"--scope"}},
		{name: "non-root", euid: 1000, want: []string{"--user", "--scope"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := systemdScopeArgsForEUID(tt.euid)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("systemdScopeArgsForEUID(%d) = %v, want %v", tt.euid, got, tt.want)
			}
		})
	}
}

func TestHostEnvPairsForSystemdUserRun(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1/bus")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1")
	t.Setenv("XDG_SESSION_ID", "3")
	t.Setenv("HOME", "/irrelevant")

	got := hostEnvPairsForSystemdUserRun()
	want := []string{
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1/bus",
		"XDG_RUNTIME_DIR=/run/user/1",
		"XDG_SESSION_ID=3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hostEnvPairsForSystemdUserRun() = %#v, want %#v", got, want)
	}
}

package circuit

import "testing"

func TestHomeZoneFor(t *testing.T) {
	cases := []struct {
		pos  Position
		want MetaPhase
	}{
		{PositionPG, MetaPhaseBk},
		{PositionSG, MetaPhasePt},
		{PositionPF, MetaPhaseFc},
		{PositionC, MetaPhaseFc},
	}
	for _, tc := range cases {
		got := HomeZoneFor(tc.pos)
		if got != tc.want {
			t.Errorf("HomeZoneFor(%s) = %q, want %q", tc.pos, got, tc.want)
		}
	}
}

func TestAgentIdentity_Tag(t *testing.T) {
	id := AgentIdentity{PersonaName: "Herald", Color: Color{Name: "Crimson"}}
	tag := id.Tag()
	if tag != "[crimson/herald]" {
		t.Errorf("Tag() = %q, want %q", tag, "[crimson/herald]")
	}
}

func TestAgentIdentity_Tag_ZeroValue(t *testing.T) {
	var id AgentIdentity
	tag := id.Tag()
	if tag != "[none/anon]" {
		t.Errorf("Tag() zero value = %q, want %q", tag, "[none/anon]")
	}
}

func TestRole_Constants(t *testing.T) {
	for _, r := range []Role{RoleWorker, RoleManager, RoleEnforcer, RoleBroker} {
		if !ValidRoles[r] {
			t.Errorf("Role %q not in ValidRoles", r)
		}
	}
	if ValidRoles["bogus"] {
		t.Error("bogus role should not be valid")
	}
}

func TestAgentIdentity_IsRole(t *testing.T) {
	id := AgentIdentity{PersonaName: "Herald", Role: RoleWorker}
	if !id.IsRole(RoleWorker) {
		t.Error("IsRole(RoleWorker) = false, want true")
	}
	if id.IsRole(RoleManager) {
		t.Error("IsRole(RoleManager) = true, want false")
	}
}

func TestAgentIdentity_HasRole(t *testing.T) {
	var id AgentIdentity
	if id.HasRole() {
		t.Error("HasRole() = true on zero-value, want false")
	}
	id.Role = RoleEnforcer
	if !id.HasRole() {
		t.Error("HasRole() = false after setting role, want true")
	}
}

func TestRole_BackwardCompat_ZeroValue(t *testing.T) {
	id := AgentIdentity{PersonaName: "Herald"}
	if id.Role != "" {
		t.Errorf("zero-value Role = %q, want empty", id.Role)
	}
	if id.HasRole() {
		t.Error("HasRole() = true on zero-value, want false")
	}
}

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
	id := AgentIdentity{Name: "Herald", ColorPref: Reservation{Color: "Crimson"}}
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

func TestAgentIdentity_Role(t *testing.T) {
	id := AgentIdentity{Name: "Herald", Role: RoleWorker}
	if id.Role != RoleWorker {
		t.Error("Role should be RoleWorker")
	}
	if id.Role == RoleManager {
		t.Error("Role should not be RoleManager")
	}
}

func TestAgentIdentity_HasRole(t *testing.T) {
	var id AgentIdentity
	if id.Role != "" {
		t.Error("zero-value Role should be empty")
	}
	id.Role = RoleEnforcer
	if id.Role == "" {
		t.Error("Role should be set after assignment")
	}
}

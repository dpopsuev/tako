package framework

import (
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// init registers a test-only persona resolver that provides the same
// personas as the persona/ package. This avoids a circular dependency
// between the root package tests and the persona sub-package.
func init() {
	DefaultPersonaResolver = testPersonaByName
	circuit.DefaultPersonaResolver = testPersonaByName
}

func testPersonaByName(name string) (Persona, bool) {
	lower := strings.ToLower(name)
	for _, p := range testAllPersonas() {
		if strings.ToLower(p.Identity.PersonaName) == lower {
			return p, true
		}
	}
	return Persona{}, false
}

func testAllPersonas() []Persona {
	return []Persona{
		{Identity: AgentIdentity{PersonaName: "Herald", Color: Color{Name: "Crimson"}, Element: ElementFire, Position: PositionPG, Alignment: AlignmentThesis, HomeZone: MetaPhaseBk}, Description: "Fast intake"},
		{Identity: AgentIdentity{PersonaName: "Seeker", Color: Color{Name: "Cerulean"}, Element: ElementWater, Position: PositionC, Alignment: AlignmentThesis, HomeZone: MetaPhaseFc}, Description: "Deep investigator"},
		{Identity: AgentIdentity{PersonaName: "Sentinel", Color: Color{Name: "Cobalt"}, Element: ElementEarth, Position: PositionPF, Alignment: AlignmentThesis, HomeZone: MetaPhaseFc}, Description: "Steady resolver"},
		{Identity: AgentIdentity{PersonaName: "Weaver", Color: Color{Name: "Amber"}, Element: ElementAir, Position: PositionSG, Alignment: AlignmentThesis, HomeZone: MetaPhasePt}, Description: "Holistic closer"},
		{Identity: AgentIdentity{PersonaName: "Challenger", Color: Color{Name: "Scarlet"}, Element: ElementFire, Position: PositionPG, Alignment: AlignmentAntithesis, HomeZone: MetaPhaseBk}, Description: "Aggressive skeptic"},
		{Identity: AgentIdentity{PersonaName: "Abyss", Color: Color{Name: "Sapphire"}, Element: ElementWater, Position: PositionC, Alignment: AlignmentAntithesis, HomeZone: MetaPhaseFc}, Description: "Deep adversary"},
		{Identity: AgentIdentity{PersonaName: "Bulwark", Color: Color{Name: "Steel"}, Element: ElementDiamond, Position: PositionPF, Alignment: AlignmentAntithesis, HomeZone: MetaPhaseFc}, Description: "Precision verifier"},
		{Identity: AgentIdentity{PersonaName: "Specter", Color: Color{Name: "Obsidian"}, Element: ElementLightning, Position: PositionSG, Alignment: AlignmentAntithesis, HomeZone: MetaPhasePt}, Description: "Fastest path to contradiction"},
	}
}

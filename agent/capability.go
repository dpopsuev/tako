package agent

// Capability is the AAI token that defines the agent's Corpus blueprint.
// Declares which Organs to attach. Issued by Lobby (PDP), enforced at Corpus assembly (PEP).
type Capability struct {
	Identity string
	Persona  Uniform
	Organs   []string
}

// WorkerCapability returns the default Capability for a Worker persona.
func WorkerCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Worker,
		Organs:   []string{"monologue", "dialog", "kanban", "andon", "workstation"},
	}
}

// ForemanCapability returns the default Capability for a Foreman persona.
func ForemanCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Foreman,
		Organs:   []string{"monologue", "dialog", "kanban", "andon"},
	}
}

// DirectorCapability returns the default Capability for a Director persona.
func DirectorCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Director,
		Organs:   []string{"monologue", "dialog", "kanban"},
	}
}

// AvatarCapability returns the default Capability for an Avatar persona.
func AvatarCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Avatar,
		Organs:   []string{"monologue", "dialog", "kanban", "andon", "canvas"},
	}
}

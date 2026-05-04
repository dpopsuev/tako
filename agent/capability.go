package agent

// Capability is the AAI token that defines the agent's services.
// Issued by Lobby (PDP), enforced at Corpus assembly (PEP).
type Capability struct {
	Identity string
	Persona  Uniform
	Services []string
}

func WorkerCapability(identity string) Capability {
	return Capability{Identity: identity, Persona: Worker, Services: []string{"dialog", "kanban", "andon"}}
}

func ForemanCapability(identity string) Capability {
	return Capability{Identity: identity, Persona: Foreman, Services: []string{"dialog", "kanban", "andon"}}
}

func DirectorCapability(identity string) Capability {
	return Capability{Identity: identity, Persona: Director, Services: []string{"dialog", "kanban", "andon"}}
}

func AvatarCapability(identity string) Capability {
	return Capability{Identity: identity, Persona: Avatar, Services: []string{"dialog", "kanban", "andon"}}
}

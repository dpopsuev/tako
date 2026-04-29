package agent

import "github.com/dpopsuev/tako/agent/organ"

// Capability is the AAI token that defines the agent's Corpus blueprint.
// Declares which Organs to attach. Issued by Lobby (PDP), enforced at Corpus assembly (PEP).
type Capability struct {
	Identity string
	Persona  Uniform
	Organs   []organ.OrganName
}

// WorkerCapability returns the default Capability for a Worker persona.
func WorkerCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Worker,
		Organs:   []organ.OrganName{organ.Dialog, organ.Kanban, organ.Andon, organ.Workstation},
	}
}

// ForemanCapability returns the default Capability for a Foreman persona.
func ForemanCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Foreman,
		Organs:   []organ.OrganName{organ.Dialog, organ.Kanban, organ.Andon},
	}
}

// DirectorCapability returns the default Capability for a Director persona.
func DirectorCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Director,
		Organs:   []organ.OrganName{organ.Dialog, organ.Kanban, organ.Andon},
	}
}

// AvatarCapability returns the default Capability for an Avatar persona.
func AvatarCapability(identity string) Capability {
	return Capability{
		Identity: identity,
		Persona:  Avatar,
		Organs:   []organ.OrganName{organ.Dialog, organ.Kanban, organ.Andon, organ.Workstation},
	}
}

package sleep

import (
	"time"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/memory"
)

// DoltDrain drains sealed Monolog letters into the Mesh as KnowledgeNodes.
type DoltDrain struct {
	monolog discourse.Monolog
}

var _ Drain = (*DoltDrain)(nil)

func NewDoltDrain(monolog discourse.Monolog) *DoltDrain {
	return &DoltDrain{monolog: monolog}
}

func (d *DoltDrain) Sweep(mesh memory.Mesh) error {
	letters := d.monolog.Letters()
	for _, letter := range letters {
		node := memory.KnowledgeNode{
			ID:        letter.From + ":" + letter.Subject,
			Content:   letter.Body,
			Tier:      memory.Knowledge,
			CreatedAt: time.Now(),
		}
		if err := mesh.AddNode(node); err != nil {
			return err
		}
	}
	return nil
}

func (d *DoltDrain) Consolidate(_ memory.Mesh) error {
	return nil
}

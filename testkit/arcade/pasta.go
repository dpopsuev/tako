package arcade

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/shell"
)

func NewPastaBolognese(ctx context.Context, sensory cerebrum.Bus) Scenario {
	adv := NewGame(map[string]any{
		"pantry":        []string{"pasta", "canned tomatoes", "garlic", "onion", "olive oil", "salt", "pepper", "carrot"},
		"fridge":        []string{"ground beef", "parmesan", "butter"},
		"stove":         "off",
		"pot":           "empty",
		"pan":           "empty",
		"cutting_board": "",
		"soffritto":     false,
		"meat":          "raw",
		"sauce":         "none",
		"water":         "none",
		"pasta":         "dry",
		"parmesan":      "whole",
		"plated":        false,
		"eaten":         false,
	}).WithSensory(sensory)

	adv.AddInstrument("look_pantry", "Check what ingredients are in the pantry", shell.ReadAction, func(s map[string]any, _ string) string {
		items, _ := s["pantry"].([]string)
		if len(items) == 0 {
			return "pantry is empty"
		}
		return fmt.Sprintf("pantry has: %v", items)
	})

	adv.AddInstrument("look_fridge", "Check what ingredients are in the fridge", shell.ReadAction, func(s map[string]any, _ string) string {
		items, _ := s["fridge"].([]string)
		if len(items) == 0 {
			return "fridge is empty"
		}
		return fmt.Sprintf("fridge has: %v", items)
	})

	adv.AddInstrument("look_kitchen", "Check the current state of everything in the kitchen", shell.ReadAction, func(s map[string]any, _ string) string {
		return fmt.Sprintf("stove: %s, pot: %s, pan: %s, cutting board: %s, water: %s, pasta: %s, meat: %s, sauce: %s",
			s["stove"], s["pot"], s["pan"], s["cutting_board"], s["water"], s["pasta"], s["meat"], s["sauce"])
	})

	adv.AddInstrument("turn_on_stove", "Turn on the stove", shell.WriteAction, func(s map[string]any, _ string) string {
		s["stove"] = "on"
		return "stove is on"
	})

	adv.AddInstrument("chop_soffritto", "Chop onion, garlic, and carrot for the soffritto base", shell.WriteAction, func(s map[string]any, _ string) string {
		s["cutting_board"] = "chopped soffritto"
		s["soffritto"] = true
		return "onion, garlic, and carrot chopped for soffritto"
	})

	adv.AddInstrument("saute_soffritto", "Saute the chopped soffritto in the pan with olive oil. Takes a moment to soften.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off, turn it on first"
		}
		if s["soffritto"] != true {
			return "chop the soffritto first (onion, garlic, carrot)"
		}
		s["pan"] = "sauteing soffritto"
		adv.StartTimer(ctx, TimerConfig{
			After: 3 * time.Second,
			Event: "the soffritto is softened and fragrant, ready for meat",
			Mutate: func(st map[string]any) { st["pan"] = "softened soffritto" },
		})
		return "soffritto is sizzling in the pan, it will be ready in a moment"
	})

	adv.AddInstrument("brown_meat", "Add ground beef to the pan and brown it. Takes a moment.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off"
		}
		pan, _ := s["pan"].(string)
		if pan != "softened soffritto" {
			return "saute the soffritto first, then add meat"
		}
		s["pan"] = "browning meat"
		adv.StartTimer(ctx, TimerConfig{
			After:   4 * time.Second,
			Event:   "meat is browned and ready for tomatoes",
			Mutate:  func(st map[string]any) { st["meat"] = "browned"; st["pan"] = "browned meat + soffritto" },
			Overdue: 6 * time.Second,
			Penalty: "WARNING: the meat is starting to burn! Add tomatoes now!",
		})
		return "ground beef is browning with the soffritto, stir occasionally"
	})

	adv.AddInstrument("add_tomatoes", "Add canned tomatoes to the pan to start the sauce. It needs to simmer.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["meat"] != "browned" {
			return "brown the meat first"
		}
		s["sauce"] = "simmering"
		s["pan"] = "bolognese simmering"
		adv.StartTimer(ctx, TimerConfig{
			After:  8 * time.Second,
			Event:  "the bolognese sauce is thick and rich, ready to combine with pasta",
			Mutate: func(st map[string]any) { st["sauce"] = "ready" },
		})
		return "tomatoes added, sauce is now simmering. This will take a while. You can prepare pasta in the meantime."
	})

	adv.AddInstrument("boil_water", "Fill pot with water and put on stove. Takes a moment to boil.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off, turn it on first"
		}
		s["pot"] = "heating"
		s["water"] = "heating"
		adv.StartTimer(ctx, TimerConfig{
			After:  5 * time.Second,
			Event:  "the water is boiling, ready for pasta",
			Mutate: func(st map[string]any) { st["water"] = "boiling"; st["pot"] = "boiling water" },
		})
		return "pot of water is on the stove, heating up"
	})

	adv.AddInstrument("cook_pasta", "Add pasta to boiling water. Takes a moment to cook al dente.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["water"] != "boiling" {
			return "water isn't boiling yet, wait for it"
		}
		s["pasta"] = "cooking"
		s["pot"] = "pasta cooking"
		adv.StartTimer(ctx, TimerConfig{
			After:   5 * time.Second,
			Event:   "pasta is al dente, drain it now!",
			Mutate:  func(st map[string]any) { st["pasta"] = "al dente" },
			Overdue: 4 * time.Second,
			Penalty: "WARNING: pasta is getting mushy! Drain immediately!",
		})
		return "pasta is in the boiling water, cooking"
	})

	adv.AddInstrument("drain_pasta", "Drain the cooked pasta", shell.WriteAction, func(s map[string]any, _ string) string {
		pasta, _ := s["pasta"].(string)
		if pasta != "al dente" && pasta != "cooking" {
			return "pasta isn't ready to drain"
		}
		s["pasta"] = "drained"
		s["pot"] = "drained pasta"
		return "pasta drained"
	})

	adv.AddInstrument("grate_parmesan", "Grate fresh parmesan cheese", shell.WriteAction, func(s map[string]any, _ string) string {
		s["parmesan"] = "grated"
		return "parmesan freshly grated"
	})

	adv.AddInstrument("combine", "Toss drained pasta with the sauce in the pan", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["pasta"] != "drained" {
			return "drain the pasta first"
		}
		if s["sauce"] != "ready" {
			return "sauce isn't ready yet, let it simmer"
		}
		s["pan"] = "pasta bolognese"
		return "pasta tossed with bolognese sauce, it's coming together beautifully"
	})

	adv.AddInstrument("plate", "Plate the pasta bolognese with parmesan on top", shell.WriteAction, func(s map[string]any, _ string) string {
		pan, _ := s["pan"].(string)
		if pan != "pasta bolognese" {
			return "combine pasta and sauce first"
		}
		s["plated"] = true
		if s["parmesan"] == "grated" {
			return "pasta bolognese plated with fresh parmesan, buon appetito"
		}
		return "pasta bolognese plated (no parmesan)"
	})

	adv.AddInstrument("eat", "Eat the plated dish", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["plated"] != true {
			return "nothing plated yet"
		}
		s["eaten"] = true
		return "delicious pasta bolognese, buon appetito"
	})

	return Scenario{
		Name: "pasta_bolognese",
		Need: "Cook pasta bolognese from scratch. Check the pantry and fridge. The proper order: chop soffritto, saute it, brown meat, add tomatoes and let sauce simmer. While the sauce simmers, boil water and cook pasta. Grate parmesan while you wait. Drain pasta when al dente. Combine pasta with sauce, plate with parmesan, eat. Things cook in real time and you will receive notifications when they are ready. The stove must be on before any cooking. Use your time wisely while things simmer.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["eaten"] == true },
	}
}

package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
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

	adv.Organ("look_pantry", "Check what ingredients are in the pantry", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			items, _ := s["pantry"].([]string)
			if len(items) == 0 {
				return organ.TextResult("pantry is empty"), nil
			}
			return organ.TextResult(fmt.Sprintf("pantry has: %v", items)), nil
		})

	adv.Organ("look_fridge", "Check what ingredients are in the fridge", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			items, _ := s["fridge"].([]string)
			if len(items) == 0 {
				return organ.TextResult("fridge is empty"), nil
			}
			return organ.TextResult(fmt.Sprintf("fridge has: %v", items)), nil
		})

	adv.Organ("look_kitchen", "Check the current state of everything in the kitchen", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult(fmt.Sprintf("stove: %s, pot: %s, pan: %s, cutting board: %s, water: %s, pasta: %s, meat: %s, sauce: %s",
				s["stove"], s["pot"], s["pan"], s["cutting_board"], s["water"], s["pasta"], s["meat"], s["sauce"])), nil
		})

	adv.Organ("turn_on_stove", "Turn on the stove", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			s["stove"] = "on"
			return organ.TextResult("stove is on"), nil
		})

	adv.Organ("chop_soffritto", "Chop onion, garlic, and carrot for the soffritto base", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			s["cutting_board"] = "chopped soffritto"
			s["soffritto"] = true
			return organ.TextResult("onion, garlic, and carrot chopped for soffritto"), nil
		})

	adv.Organ("saute_soffritto", "Saute the chopped soffritto in the pan with olive oil", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["stove"] != "on" {
				return organ.TextResult("stove is off, turn it on first"), nil
			}
			if s["soffritto"] != true {
				return organ.TextResult("chop the soffritto first (onion, garlic, carrot)"), nil
			}
			s["pan"] = "sauteing soffritto"
			adv.StartTimer(ctx, TimerConfig{
				After: 3 * time.Second,
				Event: "the soffritto is softened and fragrant, ready for meat",
				Mutate: func(st map[string]any) { st["pan"] = "softened soffritto" },
			})
			return organ.TextResult("soffritto is sizzling in the pan, it will be ready in a moment"), nil
		})

	adv.Organ("brown_meat", "Add ground beef to the pan and brown it", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["stove"] != "on" {
				return organ.TextResult("stove is off"), nil
			}
			pan, _ := s["pan"].(string)
			if pan != "softened soffritto" {
				return organ.TextResult("saute the soffritto first, then add meat"), nil
			}
			s["pan"] = "browning meat"
			adv.StartTimer(ctx, TimerConfig{
				After:   4 * time.Second,
				Event:   "meat is browned and ready for tomatoes",
				Mutate:  func(st map[string]any) { st["meat"] = "browned"; st["pan"] = "browned meat + soffritto" },
				Overdue: 6 * time.Second,
				Penalty: "WARNING: the meat is starting to burn! Add tomatoes now!",
			})
			return organ.TextResult("ground beef is browning with the soffritto, stir occasionally"), nil
		})

	adv.Organ("add_tomatoes", "Add canned tomatoes to the pan to start the sauce", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["meat"] != "browned" {
				return organ.TextResult("brown the meat first"), nil
			}
			s["sauce"] = "simmering"
			s["pan"] = "bolognese simmering"
			adv.StartTimer(ctx, TimerConfig{
				After:  8 * time.Second,
				Event:  "the bolognese sauce is thick and rich, ready to combine with pasta",
				Mutate: func(st map[string]any) { st["sauce"] = "ready" },
			})
			return organ.TextResult("tomatoes added, sauce is now simmering. This will take a while. You can prepare pasta in the meantime."), nil
		})

	adv.Organ("boil_water", "Fill pot with water and put on stove", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["stove"] != "on" {
				return organ.TextResult("stove is off, turn it on first"), nil
			}
			s["pot"] = "heating"
			s["water"] = "heating"
			adv.StartTimer(ctx, TimerConfig{
				After:  5 * time.Second,
				Event:  "the water is boiling, ready for pasta",
				Mutate: func(st map[string]any) { st["water"] = "boiling"; st["pot"] = "boiling water" },
			})
			return organ.TextResult("pot of water is on the stove, heating up"), nil
		})

	adv.Organ("cook_pasta", "Add pasta to boiling water", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["water"] != "boiling" {
				return organ.TextResult("water isn't boiling yet, wait for it"), nil
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
			return organ.TextResult("pasta is in the boiling water, cooking"), nil
		})

	adv.Organ("drain_pasta", "Drain the cooked pasta", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			pasta, _ := s["pasta"].(string)
			if pasta != "al dente" && pasta != "cooking" {
				return organ.TextResult("pasta isn't ready to drain"), nil
			}
			s["pasta"] = "drained"
			s["pot"] = "drained pasta"
			return organ.TextResult("pasta drained"), nil
		})

	adv.Organ("grate_parmesan", "Grate fresh parmesan cheese", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			s["parmesan"] = "grated"
			return organ.TextResult("parmesan freshly grated"), nil
		})

	adv.Organ("combine", "Toss drained pasta with the sauce in the pan", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["pasta"] != "drained" {
				return organ.TextResult("drain the pasta first"), nil
			}
			if s["sauce"] != "ready" {
				return organ.TextResult("sauce isn't ready yet, let it simmer"), nil
			}
			s["pan"] = "pasta bolognese"
			return organ.TextResult("pasta tossed with bolognese sauce, it's coming together beautifully"), nil
		})

	adv.Organ("plate", "Plate the pasta bolognese with parmesan on top", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			pan, _ := s["pan"].(string)
			if pan != "pasta bolognese" {
				return organ.TextResult("combine pasta and sauce first"), nil
			}
			s["plated"] = true
			if s["parmesan"] == "grated" {
				return organ.TextResult("pasta bolognese plated with fresh parmesan, buon appetito"), nil
			}
			return organ.TextResult("pasta bolognese plated (no parmesan)"), nil
		})

	adv.Organ("eat", "Eat the plated dish", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["plated"] != true {
				return organ.TextResult("nothing plated yet"), nil
			}
			s["eaten"] = true
			return organ.TextResult("delicious pasta bolognese, buon appetito"), nil
		})

	adv.Organ("check_served", "Check if the meal has been served and eaten", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["eaten"] == true {
				return organ.TextResult("the meal is served and eaten"), nil
			}
			if s["plated"] == true {
				return organ.TextResult("the meal is not served yet — it is plated, call eat"), nil
			}
			return organ.TextResult("the meal is not served yet — keep cooking"), nil
		})

	return Scenario{
		Name:      "pasta_bolognese",
		Need:      "Cook pasta bolognese from scratch. Check the pantry and fridge. The proper order: chop soffritto, saute it, brown meat, add tomatoes and let sauce simmer. While the sauce simmers, boil water and cook pasta. Grate parmesan while you wait. Drain pasta when al dente. Combine pasta with sauce, plate with parmesan, eat. Things cook in real time and you will receive notifications when they are ready. The stove must be on before any cooking. Use your time wisely while things simmer. Use check_served to verify completion.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["eaten"] == true },
		Desired:   map[string]any{"served": true},
	}
}

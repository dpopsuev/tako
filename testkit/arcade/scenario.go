package arcade

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

type Scenario struct {
	Name         string
	Need         string
	Adventure    *Game
	IsSolved     func(state map[string]any) bool
	OptimalTurns int
}

type ScenarioResult struct {
	Solved        bool
	Turns         int
	MotorCalls    int
	TotalMass     int
	OptimalTurns  int
	TokensIn      int
	TokensOut     int
	OptimalTokens int
}

// OAE returns efficiency as optimal_tokens / actual_tokens.
// Do more with less budget.
func (r ScenarioResult) OAE() float64 {
	if !r.Solved {
		return 0
	}
	actual := r.TokensIn + r.TokensOut
	if actual == 0 {
		return 0
	}
	optimal := r.OptimalTokens
	if optimal == 0 {
		optimal = r.OptimalTurns * 1000
	}
	ratio := float64(optimal) / float64(actual)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

func NewFridge() Scenario {
	adv := NewGame("fridge", map[string]any{
		"hungry": true,
		"fridge": []string{"eggs", "milk", "cheese"},
		"stove":  "off",
		"hand":   "",
		"plate":  "",
	})

	adv.AddInstrument("check_hunger", "Check if you are still hungry", organ.ReadAction, func(s map[string]any, _ string) string {
		if s["hungry"] == true {
			return "you are still hungry"
		}
		return "you are not hungry anymore"
	})

	adv.AddInstrument("look_fridge", "See what food is in the fridge", organ.ReadAction, func(s map[string]any, _ string) string {
		items, _ := s["fridge"].([]string)
		if len(items) == 0 {
			return "the fridge is empty"
		}
		return fmt.Sprintf("fridge contains: %v", items)
	})

	adv.AddInstrument("take", "Take an item from the fridge. Input: item name (e.g. 'eggs')", organ.WriteAction, func(s map[string]any, input string) string {
		input = strings.TrimSpace(strings.ToLower(input))
		items, _ := s["fridge"].([]string)
		for i, item := range items {
			if strings.EqualFold(item, input) {
				s["fridge"] = append(items[:i], items[i+1:]...)
				s["hand"] = item
				return fmt.Sprintf("you took %s from the fridge. you are now holding: %s", item, item)
			}
		}
		return fmt.Sprintf("%s is not in the fridge. available items: %v", input, items)
	})

	adv.AddInstrument("cook", "Cook what you're holding. Requires stove to be on.", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "the stove is off. call turn_on_stove first, then cook"
		}
		hand, _ := s["hand"].(string)
		if hand == "" {
			return "you're not holding anything. call take first to grab food from the fridge"
		}
		s["plate"] = fmt.Sprintf("cooked %s", hand)
		s["hand"] = ""
		return fmt.Sprintf("you cooked %s. it's on the plate now. call eat to eat it", hand)
	})

	adv.AddInstrument("turn_on_stove", "Turn on the stove", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] == "on" {
			return "stove is already on"
		}
		s["stove"] = "on"
		return "stove is now on"
	})

	adv.AddInstrument("eat", "Eat what's on the plate", organ.WriteAction, func(s map[string]any, _ string) string {
		plate, _ := s["plate"].(string)
		if plate == "" {
			return "there's nothing on the plate. cook something first"
		}
		s["plate"] = ""
		s["hungry"] = false
		return fmt.Sprintf("you ate %s. you're no longer hungry!", plate)
	})

	return Scenario{
		Name:         "fridge",
		Need:         "You are hungry. Find food in the fridge, cook it, and eat. You must turn on the stove before cooking. Use check_hunger to verify when you are no longer hungry — stop as soon as you are satisfied.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["hungry"] == false },
		OptimalTurns: 5,
	}
}


func NewDirtyRoom() Scenario {
	adv := NewGame("dirty_room", map[string]any{
		"floor":   []string{"dust", "crumbs"},
		"table":   []string{"dirty dishes"},
		"trash":   "full",
		"broom":   "closet",
		"cleaned": false,
	})

	adv.AddInstrument("look", "Look around the room to see what needs cleaning", organ.ReadAction, func(s map[string]any, _ string) string {
		floor, _ := s["floor"].([]string)
		table, _ := s["table"].([]string)
		trash, _ := s["trash"].(string)
		parts := []string{}
		if len(floor) > 0 {
			parts = append(parts, fmt.Sprintf("floor has %v", floor))
		}
		if len(table) > 0 {
			parts = append(parts, fmt.Sprintf("table has %v", table))
		}
		if trash == "full" {
			parts = append(parts, "trash can is full")
		}
		if len(parts) == 0 {
			return "the room is clean"
		}
		return fmt.Sprintf("you see: %s", fmt.Sprint(parts))
	})

	adv.AddInstrument("get_broom", "Get the broom from the closet", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["broom"] == "hand" {
			return "you already have the broom"
		}
		s["broom"] = "hand"
		return "you got the broom from the closet"
	})

	adv.AddInstrument("sweep", "Sweep the floor. Requires broom in hand.", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["broom"] != "hand" {
			return "you need the broom first, get it from the closet"
		}
		s["floor"] = []string{}
		return "floor swept clean"
	})

	adv.AddInstrument("wash_dishes", "Wash the dishes on the table", organ.WriteAction, func(s map[string]any, _ string) string {
		s["table"] = []string{}
		return "dishes washed and put away"
	})

	adv.AddInstrument("take_out_trash", "Take the trash out", organ.WriteAction, func(s map[string]any, _ string) string {
		s["trash"] = "empty"
		return "trash taken out"
	})

	adv.AddInstrument("check_done", "Check if the room is fully clean", organ.ReadAction, func(s map[string]any, _ string) string {
		floor, _ := s["floor"].([]string)
		table, _ := s["table"].([]string)
		trash, _ := s["trash"].(string)
		if len(floor) == 0 && len(table) == 0 && trash == "empty" {
			s["cleaned"] = true
			return "the room is completely clean"
		}
		remaining := []string{}
		if len(floor) > 0 {
			remaining = append(remaining, "floor still dirty")
		}
		if len(table) > 0 {
			remaining = append(remaining, "dishes still on table")
		}
		if trash == "full" {
			remaining = append(remaining, "trash still full")
		}
		return fmt.Sprintf("not done yet: %v", remaining)
	})

	return Scenario{
		Name:         "dirty_room",
		Need:         "The room is dirty. Look around, then clean everything: sweep the floor (need broom from closet first), wash dishes, take out trash.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["cleaned"] == true },
		OptimalTurns: 6,
	}
}

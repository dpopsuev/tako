package arcade

import (
	"encoding/json"
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
	OptimalSteps []string
	Desired      map[string]any
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

var emptySchema = json.RawMessage(`{"type":"object","properties":{}}`)

func NewFridge() Scenario {
	adv := NewGame(map[string]any{
		"hungry": true,
		"fridge": []string{"eggs", "milk", "cheese"},
		"stove":  "off",
		"hand":   "",
		"plate":  "",
	})

	adv.Organ("check_hunger", "Check if you are still hungry", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["hungry"] == true {
				return organ.TextResult("you are still hungry"), nil
			}
			return organ.TextResult("you are not hungry anymore"), nil
		})

	adv.Organ("look_fridge", "See what food is in the fridge", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			items, _ := s["fridge"].([]string)
			if len(items) == 0 {
				return organ.TextResult("the fridge is empty"), nil
			}
			return organ.TextResult(fmt.Sprintf("fridge contains: %v", items)), nil
		})

	adv.Organ("take", "Take an item from the fridge",
		json.RawMessage(`{"type":"object","properties":{"item":{"type":"string","description":"name of the item to take (e.g. eggs)"}},"required":["item"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Item string `json:"item"` }
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			item := strings.TrimSpace(strings.ToLower(args.Item))
			items, _ := s["fridge"].([]string)
			for i, it := range items {
				if strings.EqualFold(it, item) {
					s["fridge"] = append(items[:i], items[i+1:]...)
					s["hand"] = it
					return organ.TextResult(fmt.Sprintf("you took %s from the fridge. you are now holding: %s", it, it)), nil
				}
			}
			return organ.TextResult(fmt.Sprintf("%s is not in the fridge. available items: %v", item, items)), nil
		})

	adv.Organ("turn_on_stove", "Turn on the stove", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["stove"] == "on" {
				return organ.TextResult("stove is already on"), nil
			}
			s["stove"] = "on"
			return organ.TextResult("stove is now on"), nil
		})

	adv.Organ("cook", "Cook what you're holding. Requires stove to be on.", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["stove"] != "on" {
				return organ.TextResult("the stove is off. call turn_on_stove first, then cook"), nil
			}
			hand, _ := s["hand"].(string)
			if hand == "" {
				return organ.TextResult("you're not holding anything. call take first to grab food from the fridge"), nil
			}
			s["plate"] = fmt.Sprintf("cooked %s", hand)
			s["hand"] = ""
			return organ.TextResult(fmt.Sprintf("you cooked %s. it's on the plate now. call eat to eat it", hand)), nil
		})

	adv.Organ("eat", "Eat what's on the plate", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			plate, _ := s["plate"].(string)
			if plate == "" {
				return organ.TextResult("there's nothing on the plate. cook something first"), nil
			}
			s["plate"] = ""
			s["hungry"] = false
			return organ.TextResult(fmt.Sprintf("you ate %s. you're no longer hungry!", plate)), nil
		})

	return Scenario{
		Name:         "fridge",
		Need:         "You are hungry. Find food in the fridge, cook it, and eat. You must turn on the stove before cooking. Use check_hunger to verify when you are no longer hungry — stop as soon as you are satisfied.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["hungry"] == false },
		OptimalTurns: 3,
		OptimalSteps: []string{"look_fridge", "take", "turn_on_stove", "cook", "eat", "check_hunger"},
		Desired:      map[string]any{"hungry": false},
	}
}

func NewDirtyRoom() Scenario {
	adv := NewGame(map[string]any{
		"floor":   []string{"dust", "crumbs"},
		"table":   []string{"dirty dishes"},
		"trash":   "full",
		"broom":   "closet",
		"cleaned": false,
	})

	adv.Organ("look", "Look around the room to see what needs cleaning", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			floor, _ := s["floor"].([]string)
			table, _ := s["table"].([]string)
			trash, _ := s["trash"].(string)
			var parts []string
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
				return organ.TextResult("the room is clean"), nil
			}
			return organ.TextResult(fmt.Sprintf("you see: %s", fmt.Sprint(parts))), nil
		})

	adv.Organ("get_broom", "Get the broom from the closet", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["broom"] == "hand" {
				return organ.TextResult("you already have the broom"), nil
			}
			s["broom"] = "hand"
			return organ.TextResult("you got the broom from the closet"), nil
		})

	adv.Organ("sweep", "Sweep the floor. Requires broom in hand.", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["broom"] != "hand" {
				return organ.TextResult("you need the broom first, get it from the closet"), nil
			}
			s["floor"] = []string{}
			return organ.TextResult("floor swept clean"), nil
		})

	adv.Organ("wash_dishes", "Wash the dishes on the table", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			s["table"] = []string{}
			return organ.TextResult("dishes washed and put away"), nil
		})

	adv.Organ("take_out_trash", "Take the trash out", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			s["trash"] = "empty"
			return organ.TextResult("trash taken out"), nil
		})

	adv.Organ("check_done", "Check if the room is fully clean", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			floor, _ := s["floor"].([]string)
			table, _ := s["table"].([]string)
			trash, _ := s["trash"].(string)
			if len(floor) == 0 && len(table) == 0 && trash == "empty" {
				s["cleaned"] = true
				return organ.TextResult("the room is completely clean"), nil
			}
			var remaining []string
			if len(floor) > 0 {
				remaining = append(remaining, "floor still dirty")
			}
			if len(table) > 0 {
				remaining = append(remaining, "dishes still on table")
			}
			if trash == "full" {
				remaining = append(remaining, "trash still full")
			}
			return organ.TextResult(fmt.Sprintf("not done yet: %v", remaining)), nil
		})

	return Scenario{
		Name:         "dirty_room",
		Need:         "The room is dirty. Look around, then clean everything: sweep the floor (need broom from closet first), wash dishes, take out trash. Use check_done to verify when the room is fully clean.",
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["cleaned"] == true },
		OptimalTurns: 3,
		OptimalSteps: []string{"look", "get_broom", "sweep", "wash_dishes", "take_out_trash", "check_done"},
		Desired:      map[string]any{"cleaned": true},
	}
}

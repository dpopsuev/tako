package arcade

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

func NewHuntTheTako() Scenario {
	rooms := map[int][]int{
		1: {2, 5, 8}, 2: {1, 3, 10}, 3: {2, 4, 12}, 4: {3, 5, 7},
		5: {1, 4, 6}, 6: {5, 7, 9}, 7: {4, 6, 8}, 8: {1, 7, 9},
		9: {6, 8, 10}, 10: {2, 9, 11}, 11: {10, 12}, 12: {3, 11},
	}

	roomsAny := make(map[string]any, len(rooms))
	for k, v := range rooms {
		roomsAny[strconv.Itoa(k)] = v
	}

	adv := NewGame(map[string]any{
		"current_room": 1,
		"tako_room":    7,
		"pit_rooms":    []int{4, 11},
		"arrows":       3,
		"alive":        true,
		"tako_dead":    false,
		"rooms":        roomsAny,
		"visited":      []int{1},
	})

	adjacent := func(s map[string]any, room int) []int {
		rm, _ := s["rooms"].(map[string]any)
		adj, _ := rm[strconv.Itoa(room)].([]int)
		return adj
	}

	isAdjacent := func(s map[string]any, from, to int) bool {
		for _, r := range adjacent(s, from) {
			if r == to {
				return true
			}
		}
		return false
	}

	clues := func(s map[string]any) string {
		cur, _ := s["current_room"].(int)
		takoRoom, _ := s["tako_room"].(int)
		pitRooms, _ := s["pit_rooms"].([]int)
		var parts []string
		if isAdjacent(s, cur, takoRoom) {
			parts = append(parts, "You smell something terrible nearby.")
		}
		for _, pit := range pitRooms {
			if isAdjacent(s, cur, pit) {
				parts = append(parts, "You feel a cold draft.")
				break
			}
		}
		return strings.Join(parts, " ")
	}

	formatRooms := func(rs []int) string {
		strs := make([]string, len(rs))
		for i, r := range rs {
			strs[i] = strconv.Itoa(r)
		}
		return strings.Join(strs, ", ")
	}

	adv.Organ("look", "Look around the current cave. Shows connected caves and clues.", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("You are dead. Game over."), nil
			}
			cur, _ := s["current_room"].(int)
			adj := adjacent(s, cur)
			msg := fmt.Sprintf("You are in cave %d. Tunnels lead to caves: %s.", cur, formatRooms(adj))
			if c := clues(s); c != "" {
				msg += " " + c
			}
			return organ.TextResult(msg), nil
		})

	adv.Organ("move", "Move to an adjacent cave",
		json.RawMessage(`{"type":"object","properties":{"room":{"type":"integer","description":"cave number to move to"}},"required":["room"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("You are dead. Game over."), nil
			}
			var args struct{ Room int `json:"room"` }
			json.Unmarshal(input, &args)
			target := args.Room
			if target == 0 {
				var str string
				json.Unmarshal(input, &str)
				target, _ = strconv.Atoi(strings.TrimSpace(str))
			}
			cur, _ := s["current_room"].(int)
			if !isAdjacent(s, cur, target) {
				return organ.TextResult(fmt.Sprintf("Cave %d is not connected to cave %d. Adjacent: %s.", target, cur, formatRooms(adjacent(s, cur)))), nil
			}
			s["current_room"] = target

			visited, _ := s["visited"].([]int)
			found := false
			for _, v := range visited {
				if v == target {
					found = true
					break
				}
			}
			if !found {
				s["visited"] = append(visited, target)
			}

			takoRoom, _ := s["tako_room"].(int)
			if target == takoRoom {
				s["alive"] = false
				return organ.TextResult("The Tako got you!"), nil
			}
			pitRooms, _ := s["pit_rooms"].([]int)
			for _, pit := range pitRooms {
				if target == pit {
					s["alive"] = false
					return organ.TextResult("You fell into a pit!"), nil
				}
			}

			msg := fmt.Sprintf("You moved to cave %d.", target)
			if c := clues(s); c != "" {
				msg += " " + c
			}
			return organ.TextResult(msg), nil
		})

	adv.Organ("shoot", "Shoot an arrow into an adjacent cave",
		json.RawMessage(`{"type":"object","properties":{"room":{"type":"integer","description":"cave number to shoot into"}},"required":["room"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("You are dead. Game over."), nil
			}
			arrows, _ := s["arrows"].(int)
			if arrows <= 0 {
				s["alive"] = false
				return organ.TextResult("Out of arrows, the Tako finds you."), nil
			}
			var args struct{ Room int `json:"room"` }
			json.Unmarshal(input, &args)
			target := args.Room
			if target == 0 {
				var str string
				json.Unmarshal(input, &str)
				target, _ = strconv.Atoi(strings.TrimSpace(str))
			}
			cur, _ := s["current_room"].(int)
			if !isAdjacent(s, cur, target) {
				return organ.TextResult(fmt.Sprintf("Cave %d is not adjacent. Shoot into: %s.", target, formatRooms(adjacent(s, cur)))), nil
			}
			arrows--
			s["arrows"] = arrows
			takoRoom, _ := s["tako_room"].(int)
			if target == takoRoom {
				s["tako_dead"] = true
				return organ.TextResult("Your arrow strikes the Tako! Victory!"), nil
			}
			if arrows <= 0 {
				s["alive"] = false
				return organ.TextResult("Your arrow vanishes into the darkness. Out of arrows, the Tako finds you."), nil
			}
			return organ.TextResult(fmt.Sprintf("Your arrow vanishes into the darkness. Arrows remaining: %d.", arrows)), nil
		})

	adv.Organ("sniff", "Sniff the air to detect the Tako nearby", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("You are dead. Game over."), nil
			}
			cur, _ := s["current_room"].(int)
			takoRoom, _ := s["tako_room"].(int)
			if isAdjacent(s, cur, takoRoom) {
				adj := adjacent(s, cur)
				return organ.TextResult(fmt.Sprintf("The terrible smell is coming from one of: %s.", formatRooms(adj))), nil
			}
			return organ.TextResult("The air is clear. No smell detected."), nil
		})

	adv.Organ("status", "Check your current status: cave, arrows, visited caves", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("You are dead. Game over."), nil
			}
			cur, _ := s["current_room"].(int)
			arrows, _ := s["arrows"].(int)
			visited, _ := s["visited"].([]int)
			return organ.TextResult(fmt.Sprintf("Current cave: %d. Arrows remaining: %d. Caves visited: %s.", cur, arrows, formatRooms(visited))), nil
		})

	adv.Organ("check_caught", "Check if the Tako has been caught", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["tako_dead"] == true {
				return organ.TextResult("the tako is caught — victory!"), nil
			}
			if s["alive"] != true {
				return organ.TextResult("you are dead — the tako is not caught"), nil
			}
			return organ.TextResult("the tako is not caught yet — keep hunting"), nil
		})

	return Scenario{
		Name: "hunt_the_tako",
		Need: "You are hunting a Tako in a network of 12 dark caves. The Tako lurks in one cave. " +
			"Two caves have bottomless pits. You have 3 arrows. Move between connected caves. " +
			"If you smell something terrible, the Tako is in an adjacent cave — shoot an arrow there. " +
			"If you feel a draft, a pit is adjacent — avoid it. " +
			"You cannot see the Tako or pits directly. Use clues to deduce their locations. " +
			"Use check_caught to verify when the Tako is caught.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["tako_dead"] == true },
		Desired:   map[string]any{"caught": true},
	}
}

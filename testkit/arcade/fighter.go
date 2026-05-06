package arcade

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/capability"
)

// NewTakoFighter creates a 1v1 combat game.
// Rock-paper-scissors core: attack beats special, special beats defend, defend beats attack.
// Each player has 100 HP. Actions resolve simultaneously per round.
func NewTakoFighter() (*Game, *PlayerView, *PlayerView) {
	game := NewGame(map[string]any{
		"p1_hp":     100,
		"p2_hp":     100,
		"p1_action": "",
		"p2_action": "",
		"round":     0,
		"winner":    "",
	})

	game.AddInstrument("p1_attack", "Player 1: Attack the opponent. Beats special, loses to defend.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p1_action"] = "attack"
		return resolveRound(s)
	})

	game.AddInstrument("p1_defend", "Player 1: Defend against attacks. Beats attack, loses to special.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p1_action"] = "defend"
		return resolveRound(s)
	})

	game.AddInstrument("p1_special", "Player 1: Use special move. Beats defend, loses to attack.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p1_action"] = "special"
		return resolveRound(s)
	})

	game.AddInstrument("p1_check_hp", "Player 1: Check both players' HP and round number", capability.ReadAction, func(s map[string]any, _ string) string {
		return fmt.Sprintf("Round %d | Your HP: %d | Opponent HP: %d", s["round"], s["p1_hp"], s["p2_hp"])
	})

	game.AddInstrument("p2_attack", "Player 2: Attack the opponent. Beats special, loses to defend.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p2_action"] = "attack"
		return resolveRound(s)
	})

	game.AddInstrument("p2_defend", "Player 2: Defend against attacks. Beats attack, loses to special.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p2_action"] = "defend"
		return resolveRound(s)
	})

	game.AddInstrument("p2_special", "Player 2: Use special move. Beats defend, loses to attack.", capability.WriteAction, func(s map[string]any, _ string) string {
		s["p2_action"] = "special"
		return resolveRound(s)
	})

	game.AddInstrument("p2_check_hp", "Player 2: Check both players' HP and round number", capability.ReadAction, func(s map[string]any, _ string) string {
		return fmt.Sprintf("Round %d | Your HP: %d | Opponent HP: %d", s["round"], s["p2_hp"], s["p1_hp"])
	})

	p1View := NewPlayerView(game, "p1", []string{"p1_attack", "p1_defend", "p1_special", "p1_check_hp"})
	p2View := NewPlayerView(game, "p2", []string{"p2_attack", "p2_defend", "p2_special", "p2_check_hp"})

	return game, p1View, p2View
}

func resolveRound(s map[string]any) string {
	p1 := s["p1_action"].(string)
	p2 := s["p2_action"].(string)
	if p1 == "" || p2 == "" {
		return "waiting for both players to choose..."
	}

	round, _ := s["round"].(int)
	round++
	s["round"] = round
	s["p1_action"] = ""
	s["p2_action"] = ""

	p1Dmg, p2Dmg := resolveDamage(p1, p2)

	p1HP, _ := s["p1_hp"].(int)
	p2HP, _ := s["p2_hp"].(int)
	p1HP -= p1Dmg
	p2HP -= p2Dmg
	if p1HP < 0 {
		p1HP = 0
	}
	if p2HP < 0 {
		p2HP = 0
	}
	s["p1_hp"] = p1HP
	s["p2_hp"] = p2HP

	result := fmt.Sprintf("Round %d: P1=%s vs P2=%s. ", round, p1, p2)
	if p1Dmg > 0 {
		result += fmt.Sprintf("P1 takes %d damage. ", p1Dmg)
	}
	if p2Dmg > 0 {
		result += fmt.Sprintf("P2 takes %d damage. ", p2Dmg)
	}
	if p1Dmg == 0 && p2Dmg == 0 {
		result += "Clash! No damage dealt. "
	}
	result += fmt.Sprintf("HP: P1=%d P2=%d", p1HP, p2HP)

	if p2HP <= 0 {
		s["winner"] = "p1"
		result += ". P1 wins!"
	} else if p1HP <= 0 {
		s["winner"] = "p2"
		result += ". P2 wins!"
	}

	return result
}

func resolveDamage(p1, p2 string) (p1Takes, p2Takes int) {
	p1 = strings.ToLower(p1)
	p2 = strings.ToLower(p2)

	if p1 == p2 {
		return 0, 0
	}

	dmg := 25

	switch {
	case p1 == "attack" && p2 == "special":
		return 0, dmg
	case p1 == "attack" && p2 == "defend":
		return dmg, 0
	case p1 == "defend" && p2 == "attack":
		return 0, dmg
	case p1 == "defend" && p2 == "special":
		return dmg, 0
	case p1 == "special" && p2 == "defend":
		return 0, dmg
	case p1 == "special" && p2 == "attack":
		return dmg, 0
	}
	return 0, 0
}

// NewTakoFighterMatch creates a ready-to-run Match for the fighter game.
func NewTakoFighterMatch() *Match {
	game, p1View, p2View := NewTakoFighter()

	match := NewMatch(game, func(s map[string]any) bool {
		return s["winner"] != ""
	}, 10)

	match.AddPlayer("fighter_1", p1View, reactivity.Catalyst{
		Need:     "You are Player 1 in a fighting game. Each round, choose attack, defend, or special. Attack beats special, special beats defend, defend beats attack. Check your HP with p1_check_hp. Reduce your opponent's HP to 0 to win.",
		Desired: map[string]any{"winner": true},
	})

	match.AddPlayer("fighter_2", p2View, reactivity.Catalyst{
		Need:     "You are Player 2 in a fighting game. Each round, choose attack, defend, or special. Attack beats special, special beats defend, defend beats attack. Check your HP with p2_check_hp. Reduce your opponent's HP to 0 to win.",
		Desired: map[string]any{"winner": true},
	})

	return match
}

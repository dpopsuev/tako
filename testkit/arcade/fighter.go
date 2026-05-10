package arcade

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
)

func NewTakoFighter() (*Game, *PlayerView, *PlayerView) {
	game := NewGame(map[string]any{
		"p1_hp":     100,
		"p2_hp":     100,
		"p1_action": "",
		"p2_action": "",
		"round":     0,
		"winner":    "",
	})

	for _, p := range []struct{ prefix, label string }{{"p1", "Player 1"}, {"p2", "Player 2"}} {
		prefix, label := p.prefix, p.label

		game.Organ(prefix+"_attack", label+": Attack the opponent. Beats special, loses to defend.", emptySchema, organ.WriteAction,
			func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
				s[prefix+"_action"] = "attack"
				return organ.TextResult(resolveRound(s)), nil
			})

		game.Organ(prefix+"_defend", label+": Defend against attacks. Beats attack, loses to special.", emptySchema, organ.WriteAction,
			func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
				s[prefix+"_action"] = "defend"
				return organ.TextResult(resolveRound(s)), nil
			})

		game.Organ(prefix+"_special", label+": Use special move. Beats defend, loses to attack.", emptySchema, organ.WriteAction,
			func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
				s[prefix+"_action"] = "special"
				return organ.TextResult(resolveRound(s)), nil
			})

		game.Organ(prefix+"_check_hp", label+": Check both players' HP and round number", emptySchema, organ.ReadAction,
			func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
				myHP := s[prefix+"_hp"]
				oppPrefix := "p2"
				if prefix == "p2" {
					oppPrefix = "p1"
				}
				oppHP := s[oppPrefix+"_hp"]
				return organ.TextResult(fmt.Sprintf("Round %d | Your HP: %v | Opponent HP: %v", s["round"], myHP, oppHP)), nil
			})
	}

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

func NewTakoFighterMatch() *Match {
	game, p1View, p2View := NewTakoFighter()

	match := NewMatch(game, func(s map[string]any) bool {
		return s["winner"] != ""
	}, 10)

	match.AddPlayer("fighter_1", p1View, reactivity.Catalyst{
		Need:    "You are Player 1 in a fighting game. Each round, choose attack, defend, or special. Attack beats special, special beats defend, defend beats attack. Check your HP with p1_check_hp. Reduce your opponent's HP to 0 to win.",
		Desired: map[string]any{"winner": true},
	})

	match.AddPlayer("fighter_2", p2View, reactivity.Catalyst{
		Need:    "You are Player 2 in a fighting game. Each round, choose attack, defend, or special. Attack beats special, special beats defend, defend beats attack. Check your HP with p2_check_hp. Reduce your opponent's HP to 0 to win.",
		Desired: map[string]any{"winner": true},
	})

	return match
}

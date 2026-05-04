package arcade

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/shell"
)

func NewTakoyaki(ctx context.Context, sensory cerebrum.Bus) Scenario {
	adv := NewGame(map[string]any{
		"stove":           "off",
		"grill":           "empty",
		"fryer":           "empty",
		"cutting_board":   "empty",
		"orders_pending":  []string{},
		"orders_served":   0,
		"orders_burned":   0,
		"orders_target":   5,
		"fire":            false,
		"batter":          0,
		"filling_ready":   false,
		"takoyaki_cooking": false,
		"takoyaki_done":   0,
		"tempura_cooking": false,
		"tempura_done":    0,
		"rice_cooking":    false,
		"rice_done":       0,
	}).WithSensory(sensory)

	go func() {
		recipes := []string{"takoyaki", "tempura", "rice_bowl"}
		for i := 0; i < 7; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(3+rand.Intn(5)) * time.Second):
			}
			recipe := recipes[rand.Intn(len(recipes))]
			adv.mu.Lock()
			pending, _ := adv.state["orders_pending"].([]string)
			adv.state["orders_pending"] = append(pending, recipe)
			adv.mu.Unlock()
			sensory.Send(ctx, cerebrum.Event{
				ID:        fmt.Sprintf("order-%d", time.Now().UnixNano()),
				Kind:      "sensory.order",
				Source:    "kitchen",
				Payload:   []byte(fmt.Sprintf("NEW ORDER: %s (order #%d)", recipe, i+1)),
				CreatedAt: time.Now(),
			})
		}
	}()

	adv.AddInstrument("look_kitchen", "Check the current state of everything in the kitchen", shell.ReadAction, func(s map[string]any, _ string) string {
		pending, _ := s["orders_pending"].([]string)
		return fmt.Sprintf("stove: %s, grill: %s, fryer: %s, cutting board: %s, fire: %v, pending orders: %v, served: %d, burned: %d, batter: %d, filling ready: %v, takoyaki done: %d, tempura done: %d, rice done: %d",
			s["stove"], s["grill"], s["fryer"], s["cutting_board"], s["fire"], pending, s["orders_served"], s["orders_burned"], s["batter"], s["filling_ready"], s["takoyaki_done"], s["tempura_done"], s["rice_done"])
	})

	adv.AddInstrument("turn_on_stove", "Turn on the stove and grill", shell.WriteAction, func(s map[string]any, _ string) string {
		s["stove"] = "on"
		return "stove and grill are on, ready to cook"
	})

	adv.AddInstrument("extinguish", "Put out a kitchen fire", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["fire"] != true {
			return "no fire to extinguish"
		}
		s["fire"] = false
		return "fire extinguished, back to cooking"
	})

	adv.AddInstrument("prep_batter", "Mix takoyaki batter (flour, eggs, dashi)", shell.WriteAction, func(s map[string]any, _ string) string {
		s["batter"] = 6
		return "batter mixed, enough for 6 takoyaki balls"
	})

	adv.AddInstrument("prep_filling", "Chop octopus, green onion, pickled ginger for filling", shell.WriteAction, func(s map[string]any, _ string) string {
		s["cutting_board"] = "chopped filling"
		s["filling_ready"] = true
		return "filling chopped and ready"
	})

	adv.AddInstrument("cook_takoyaki", "Pour batter into takoyaki grill with filling. Takes a moment to cook.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off, turn it on first"
		}
		if s["fire"] == true {
			return "kitchen is on fire! extinguish first"
		}
		batter, _ := s["batter"].(int)
		if batter < 3 {
			return "not enough batter, prep more"
		}
		if s["filling_ready"] != true {
			return "filling not prepped yet"
		}
		s["batter"] = batter - 3
		s["grill"] = "cooking takoyaki"
		s["takoyaki_cooking"] = true
		adv.StartTimer(ctx, TimerConfig{
			After: 4 * time.Second,
			Event: "takoyaki are golden and round, ready to serve!",
			Mutate: func(st map[string]any) {
				done, _ := st["takoyaki_done"].(int)
				st["takoyaki_done"] = done + 3
				st["grill"] = "done"
				st["takoyaki_cooking"] = false
			},
			Overdue: 6 * time.Second,
			Penalty: "FIRE! The takoyaki burned and the grill caught fire!",
		})
		return "takoyaki batter poured into grill with filling, cooking 3 balls"
	})

	adv.AddInstrument("cook_tempura", "Fry tempura vegetables in the deep fryer. Takes a moment.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off"
		}
		if s["fire"] == true {
			return "kitchen is on fire! extinguish first"
		}
		s["fryer"] = "frying tempura"
		s["tempura_cooking"] = true
		adv.StartTimer(ctx, TimerConfig{
			After: 3 * time.Second,
			Event: "tempura is crispy and golden, ready to plate!",
			Mutate: func(st map[string]any) {
				done, _ := st["tempura_done"].(int)
				st["tempura_done"] = done + 1
				st["fryer"] = "done"
				st["tempura_cooking"] = false
			},
			Overdue: 5 * time.Second,
			Penalty: "FIRE! The tempura oil caught fire!",
		})
		return "tempura vegetables dropped in fryer, cooking"
	})

	adv.AddInstrument("cook_rice", "Start cooking rice on the stove. Takes a moment.", shell.WriteAction, func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off"
		}
		if s["fire"] == true {
			return "kitchen is on fire! extinguish first"
		}
		s["rice_cooking"] = true
		adv.StartTimer(ctx, TimerConfig{
			After: 5 * time.Second,
			Event: "rice is fluffy and ready!",
			Mutate: func(st map[string]any) {
				done, _ := st["rice_done"].(int)
				st["rice_done"] = done + 2
				st["rice_cooking"] = false
			},
		})
		return "rice on the stove, cooking"
	})

	adv.AddInstrument("serve", "Serve a completed dish to fill a pending order. Input: dish name (takoyaki, tempura, rice_bowl)", shell.WriteAction, func(s map[string]any, input string) string {
		if s["fire"] == true {
			return "kitchen is on fire! extinguish before serving"
		}
		pending, _ := s["orders_pending"].([]string)
		if len(pending) == 0 {
			return "no pending orders"
		}

		found := -1
		for i, order := range pending {
			if order == input {
				found = i
				break
			}
		}
		if found == -1 {
			return fmt.Sprintf("no pending order for %s, pending: %v", input, pending)
		}

		switch input {
		case "takoyaki":
			done, _ := s["takoyaki_done"].(int)
			if done < 3 {
				return fmt.Sprintf("not enough takoyaki ready (have %d, need 3)", done)
			}
			s["takoyaki_done"] = done - 3
		case "tempura":
			done, _ := s["tempura_done"].(int)
			if done < 1 {
				return "no tempura ready"
			}
			s["tempura_done"] = done - 1
		case "rice_bowl":
			done, _ := s["rice_done"].(int)
			if done < 1 {
				return "no rice ready"
			}
			s["rice_done"] = done - 1
		default:
			return fmt.Sprintf("unknown dish: %s", input)
		}

		s["orders_pending"] = append(pending[:found], pending[found+1:]...)
		served, _ := s["orders_served"].(int)
		s["orders_served"] = served + 1
		return fmt.Sprintf("%s served! (%d/%d orders complete)", input, served+1, s["orders_target"])
	})

	adv.AddInstrument("check_orders", "Check the current order queue", shell.ReadAction, func(s map[string]any, _ string) string {
		pending, _ := s["orders_pending"].([]string)
		served, _ := s["orders_served"].(int)
		target, _ := s["orders_target"].(int)
		if len(pending) == 0 {
			return fmt.Sprintf("no pending orders, %d/%d served", served, target)
		}
		return fmt.Sprintf("pending: %v, served: %d/%d", pending, served, target)
	})

	return Scenario{
		Name: "takoyaki",
		Need: "You are a chef in a busy takoyaki kitchen. Orders arrive in real time on a queue. You must prep ingredients, cook dishes on the grill/fryer/stove, and serve them before they pile up. Three dishes: takoyaki (needs batter + filling, cooks on grill), tempura (fries in fryer), rice_bowl (cooks on stove). Turn on the stove first. Watch for timer notifications when food is ready. If you leave food too long it catches fire — extinguish immediately. Serve 5 orders to win. You will receive order notifications as they arrive.",
		Adventure: adv,
		IsSolved: func(s map[string]any) bool {
			served, _ := s["orders_served"].(int)
			target, _ := s["orders_target"].(int)
			return served >= target
		},
	}
}

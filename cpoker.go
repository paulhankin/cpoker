// Package cpoker provides evaluators and training support for Chinese Poker.
// The rules of the game and the structure of this code is described
// in http://paulhankin.github.io/ChinesePoker
//
// Basic usage of this package, which uses the trained coefficients
// in the coefficients.data file in this directory is as follows:
//
// cards := <construct 13 card slice using github.com/paulhankin/poker>
// if hero, err = cpoker.LoadSampledEvaluator(*fromFile); err != nil {
//     log.Fatalf("failed to load evaluator: %s", err)
// }
// h, _ : =cpoker.Play(cards, hero)
// fmt.Println(h) // Shows the played front, middle and back hands.
package cpoker

import (
	"fmt"
	"math/rand"
	"reflect"

	"github.com/paulhankin/poker"
)

// A Hand is a full Chinese poker hand.
type Hand struct {
	Front  [3]poker.Card
	Middle [5]poker.Card
	Back   [5]poker.Card
}

func (h *Hand) String() string {
	fd, _ := poker.Describe(h.Front[:])
	md, _ := poker.Describe(h.Middle[:])
	bd, _ := poker.Describe(h.Back[:])
	return fmt.Sprintf("%v (%s), %v (%s), %v (%s)", h.Front, fd, h.Middle, md, h.Back, bd)
}

// A HandEvaluator scores a Chinese poker hand.
type HandEvaluator interface {
	// Evaluator should, given cards, return a function that can
	// evaluate hands created from those cards.
	Evaluator(c []poker.Card) func(evf, evm, evb int16) float64
}

// A MaxProdEvaluator evaluates hands by considering the product of the ranks
// of the three parts. This is a simple strategy, but doesn't play well.
type MaxProdEvaluator struct{}

// Evaluator returns the function that evaluates hands based on the product
// of their ranks.
func (MaxProdEvaluator) Evaluator(_ []poker.Card) func(evf, evm, evb int16) float64 {
	return evaluateProdHand
}

func evaluateProdHand(f, m, b int16) float64 {
	return float64(f) * float64(m) * float64(b) / (poker.ScoreMax * poker.ScoreMax * poker.ScoreMax)
}

func next3(ix *[3]int) bool {
	for i := 0; i < 2; i++ {
		ix[i]++
		if ix[i] != ix[i+1] {
			return true
		}
		ix[i] = i
	}
	ix[2]++
	return ix[2] < 13
}

func next4(ix *[5]int) bool {
	for i := 1; i < 4; i++ {
		ix[i]++
		if ix[i] != ix[i+1] {
			return true
		}
		ix[i] = i - 1
	}
	ix[4]++
	return ix[4] < 9
}

// EvalStats are data from playing a single hand.
type EvalStats struct {
	Hands            int // How many evals we did
	StrongFront      int // How many times the front was too strong
	BackEqualsMiddle int // How many times the back was equal to the middle
}

// Play takes 13 cards and returns the hand for which
// the evaluator returns the largest value.
func Play(c []poker.Card, he HandEvaluator) (Hand, EvalStats) {
	stats := EvalStats{}
	evaluator := he.Evaluator(c)
	maxima := make([][3]int16, 0, 128)
	best, bestEV := Hand{}, -9999999.9
	fIdx := [3]int{-1, 1, 2} // Which cards go in front
	for next3(&fIdx) {
		front := [3]poker.Card{c[fIdx[0]], c[fIdx[1]], c[fIdx[2]]}
		ef := poker.Eval3(&front)
		bIdx := [5]int{-1, -1, 1, 2, 3}
		for next4(&bIdx) {
			var back, middle [5]poker.Card
			f, b := 0, 0
			for i := 0; i < 13; i++ {
				if f < 3 && fIdx[f] == i {
					f++
				} else if b < 5 && i == bIdx[b]+f+1 {
					back[b] = c[i]
					b++
				} else {
					middle[i-f-b] = c[i]
				}
			}
			eb := poker.Eval5(&back)
			em := poker.Eval5(&middle)
			if ef >= em || ef >= eb {
				stats.StrongFront++
				continue
			}
			if em == eb {
				stats.BackEqualsMiddle++
				continue
			}
			dominated := false
			sem, seb := em, eb
			if em > eb {
				sem, seb = eb, em
			}
			for i := 0; i < len(maxima); i++ {
				if maxima[i][0] >= ef && maxima[i][1] >= sem && maxima[i][2] >= seb {
					dominated = true
					break
				}
				if maxima[i][0] <= ef && maxima[i][1] <= sem && maxima[i][2] <= seb {
					// Current hand dominates previously found maxima. Remove it.
					maxima[i] = maxima[len(maxima)-1]
					maxima = maxima[:len(maxima)-1]
				}
			}
			if dominated {
				continue
			}
			if len(maxima) < cap(maxima) {
				maxima = append(maxima, [3]int16{ef, sem, seb})
			}
			var ev float64
			if em > eb {
				ev = evaluator(ef, eb, em)
			} else {
				ev = evaluator(ef, em, eb)
			}
			stats.Hands++
			if ev >= bestEV {
				bestEV = ev
				best.Front = front
				if em > eb {
					best.Middle = back
					best.Back = middle
				} else {
					best.Middle = middle
					best.Back = back
				}
			}
		}
	}
	return best, stats
}

// A Comparison is aggregated statistics from matching two
// players ("hero" and "villain").
type Comparison struct {
	Played        int     // The number of hands played
	EVPerHand     float64 // Expectation of hero per hand
	HeroScoops    int     // How many time the hero won all three hands
	VillainScoops int     // How many times the villain won all three hands
	Same          int     // How many times the hero and villain played the hand the same way
}

// CompareEvaluators matches the two evaluators against each other on
// n random hands. Aggregate statistics are returned.
func CompareEvaluators(hero, villain HandEvaluator, n int, prEvery int) Comparison {
	cards := append([]poker.Card{}, poker.Cards...)
	result := Comparison{}
	total := float64(0)
	for hand := 0; hand < n; hand++ {
		for i := 0; i < 26; i++ {
			j := rand.Intn(52-i) + i
			cards[i], cards[j] = cards[j], cards[i]
		}
		hc := cards[:13]
		vc := cards[13:26]
		hero0, _ := Play(hc, hero)
		hero1, _ := Play(vc, hero)
		vill0, _ := Play(vc, villain)
		vill1, _ := Play(hc, villain)
		score0 := CompareHands(&hero0, &vill0)
		score1 := CompareHands(&hero1, &vill1)
		result.Played += 2
		if reflect.DeepEqual(hero0, vill1) {
			result.Same += 1
		}
		if reflect.DeepEqual(hero1, vill0) {
			result.Same += 1
		}
		total += float64(score0 + score1)
		result.EVPerHand = total / float64(result.Played)
		if score0 == 4 {
			result.HeroScoops++
		} else if score0 == -4 {
			result.VillainScoops++
		}
		if score1 == 4 {
			result.HeroScoops++
		} else if score1 == -4 {
			result.VillainScoops++
		}
		if hand%prEvery == 0 {
			fmt.Printf("hand %d\n", hand)
			fmt.Printf("  %s\n", &hero0)
			fmt.Printf("  %s\n", &vill0)
			fmt.Printf("Played the other way:\n")
			fmt.Printf("  %s\n", &hero1)
			fmt.Printf("  %s\n", &vill1)
			fmt.Printf("score: %d + %d\n", score0, score1)
			fmt.Printf("comparison:\n%#v\n\n", result)
		}
	}
	return result
}

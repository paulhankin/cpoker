package cpoker

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/paulhankin/poker/v2/poker"
)

func BenchmarkPlayProd(b *testing.B) {
	cards := append([]poker.Card{}, poker.Cards...)
	he := MaxProdEvaluator{}
	for count := 0; count < b.N; count++ {
		for i := 0; i < 13; i++ {
			j := rand.Intn(52-i) + i
			cards[i], cards[j] = cards[j], cards[i]
		}
		h, max := Play(cards[:13], he)
		//fmt.Printf("%v: %#v\n", h, max)
		_, _ = h, max
	}
}

func BenchmarkPlayRollout(b *testing.B) {
	runtime.GOMAXPROCS(8)
	cards := append([]poker.Card{}, poker.Cards...)
	he := &RolloutEvaluator{
		Opponent: MaxProdEvaluator{},
		N:        10000,
	}
	for count := 0; count < b.N; count++ {
		for i := 0; i < 13; i++ {
			j := rand.Intn(52-i) + i
			cards[i], cards[j] = cards[j], cards[i]
		}
		h, max := Play(cards[:13], he)
		//fmt.Printf("%v: %#v\n", h, max)
		_, _ = h, max
	}
}

func BenchmarkPlayComparison(b *testing.B) {
	runtime.GOMAXPROCS(4)
	rand.Seed(time.Now().UTC().UnixNano())
	var hero, villain HandEvaluator
	hero = MaxProdEvaluator{}
	for iterations := 0; iterations < 10; iterations++ {
		hero, villain = NewTrainedSampledEvaluator(hero, 1000), hero
		b.Log("iteration", iterations)
	}
	b.Log("preparing rollout evaluator")
	re := &RolloutEvaluator{PreRollout: true, Separable: true, Opponent: hero, N: 20000}
	re.Init()
	b.Log("running comparison")
	hero, villain = re, hero
	comparison := CompareEvaluators(hero, villain, 1000, 500)
	fmt.Println(comparison)
}

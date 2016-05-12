package cpoker

import (
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"
)

func BenchmarkPlayProd(b *testing.B) {
	cards := append([]Card{}, Cards...)
	he := MaxProdEvaluator{}
	for count := 0; count < b.N; count++ {
		for i := 0; i < 13; i++ {
			j := rand.Intn(52-i) + i
			cards[i], cards[j] = cards[j], cards[i]
		}
		h, max := Play(cards[:13], he)
		fmt.Printf("%v: %#v\n", h, max)
		_, _ = h, max
	}
}

func BenchmarkPlayRollout(b *testing.B) {
	runtime.GOMAXPROCS(8)
	cards := append([]Card{}, Cards...)
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
		fmt.Printf("%v: %#v\n", h, max)
		_, _ = h, max
	}
}

func BenchmarkPlayComparison(b *testing.B) {
	runtime.GOMAXPROCS(4)
	rand.Seed(time.Now().UTC().UnixNano())
	var hero, villain HandEvaluator
	hero = MaxProdEvaluator{}
	for iterations := 0; iterations < 20; iterations++ {
		hero, villain = NewTrainedSampledEvaluator(hero, 1000), hero
		fmt.Println("iteration", iterations)
	}
	re := &RolloutEvaluator{PreRollout: true, Separable: true, Opponent: hero, N: 100000}
	re.Init()
	hero, villain = re, hero
	comparison := CompareEvaluators(hero, villain, 100000, 100)
	fmt.Println(comparison)
}

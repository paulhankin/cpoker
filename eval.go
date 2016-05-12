// Package cpoker provides functions for Chinese Poker simulation.
package cpoker

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// A Card is a single playing card.
// The top two bits are the suit, and the lowest 6 bits
// store the (r-1)th prime number.
// This representation enables fast hand evaluation.
// Heavily based on the ideas from Cactus Pete's poker
// hand evaluator, which can be found here:
// See http://www.suffecool.net/poker/evaluator.html
type Card uint16

// Suit returns the suit of a card.
func (c Card) Suit() Suit {
	switch c >> 6 {
	case 1:
		return Club
	case 2:
		return Diamond
	case 4:
		return Heart
	case 8:
		return Spade
	}
	log.Fatalf("unknown suit: %d", c>>6)
	return Club
}

// Rank returns the rank of a card.
func (c Card) Rank() Rank {
	r := uint16(c & 0x3f)
	for pr := Rank(1); pr <= 13; pr++ {
		if primes[pr-1] == r {
			return pr
		}
	}
	return 0
}

func (c Card) String() string {
	return c.Suit().String() + c.Rank().String()
}

// A Suit is a suit: clubs, diamonds, hearts or spades.
type Suit uint8

const (
	Club    = Suit(0)
	Diamond = Suit(1)
	Heart   = Suit(2)
	Spade   = Suit(3)
)

var suits = map[Suit]string{
	Club:    "C",
	Diamond: "D",
	Heart:   "H",
	Spade:   "S",
}

func (s Suit) String() string {
	return suits[s]
}

// A Rank describes the rank of a card: A23456789TJQK.
// Ace is 1, King is 13.
type Rank int

func (r Rank) String() string {
	return "A23456789TJQK"[r-1 : r]
}

var primes = []uint16{
	2, 3, 5, 7, 11, 13, 17, 23, 29, 31, 37, 41, 43,
}

// MakeCard constructs a card from a suit and rank.
func MakeCard(s Suit, r Rank) (Card, error) {
	if s > 3 || r == 0 || r > 13 {
		return 0, fmt.Errorf("illegal card %d %d", s, r)
	}
	return Card(uint16(1<<(6+s)) | primes[r-1]), nil
}

// NameToCard maps card names (for example, "C8" or "HA") to a card value.
var NameToCard = map[string]Card{}

// Cards is a full deck of all cards. Sorted by suit and then rank.
var Cards []Card

func init() {
	for s := Suit(0); s <= Suit(3); s++ {
		for r := Rank(1); r <= Rank(13); r++ {
			c, err := MakeCard(s, r)
			if err != nil {
				log.Fatalf("Cards construction failed: %s", err)
			}
			NameToCard[c.String()] = c
			Cards = append(Cards, c)
		}
	}
}

type eval struct {
	desc string
	rank int
}

// evalScore condenses the strength of a hand given its type
// and cards into a single int.
func evalScore(f string, v int, c ...int) eval {
	r := v
	for i := 0; i < 5; i++ {
		r *= 16
		if i < len(c) {
			r += c[i]
		}
	}
	args := make([]interface{}, len(c))
	for i := range c {
		if c[i] == 0 {
			args[i] = ""
			continue
		}
		args[i] = Rank((c[i]-1)%13 + 1)
	}
	return eval{
		desc: fmt.Sprintf(f, args...),
		rank: r,
	}
}

// find picks the nth highest rank of r which is equal to k,
// returning a number which is higher for higher cards.
// Returns 0 if there is none.
func find(k int, r map[Rank]int, n int) int {
	for i := 0; i < 13; i++ {
		rank := Rank(14 - i)
		if i == 0 {
			rank = 1
		}
		if r[rank] == k {
			if n == 0 {
				return 14 - i
			}
			n--
		}
	}
	return 0
}

func find1(r map[Rank]int, n int) int {
	return find(1, r, n)
}

func find2(r map[Rank]int, n int) int {
	return find(2, r, n)
}

func isFlush(c []Card) bool {
	if len(c) != 5 {
		return false
	}
	for _, ci := range c {
		if ci.Suit() != c[0].Suit() {
			return false
		}
	}
	return true
}

// Describe fully describes a 3 or 5 card poker hand.
func Describe(c []Card) (string, error) {
	eval, err := evalSlow(c, true)
	if err != nil {
		return "", err
	}
	// The description of a three-card hand includes trailing dashes
	// where kickers are missing. We remove them.
	return strings.TrimRight(eval.desc, "-"), nil
}

// Describe describes a 3 or 5 card poker hand with enough detail
// to compare it to another chinese poker hand.
func DescribeShort(c []Card) (string, error) {
	eval, err := evalSlow(c, false)
	if err != nil {
		return "", err
	}
	// The description of a three-card hand includes trailing dashes
	// where kickers are missing. We remove them.
	return strings.TrimRight(eval.desc, "-"), nil
}

// eval5Slow evaluates a 3- or 5- card poker hand.
// The result is a number which can be compared
// with other hand's evaluations to correctly rank them as poker
// hands.
// If "replace" is false, then details are dropped of hands that can't be
// used for comparing against hands which are drawn from the same
// deck (for example: the kickers with trip aces don't matter).
//
// This function is used to build tables for fast hand evaluation.
func evalSlow(c []Card, replace bool) (eval, error) {
	flush := isFlush(c)
	ranks := map[Rank]int{}
	dupes := [6]int{}  // uniqs, pairs, trips, quads, quins
	str8s := [13]int{} // finds straights
	str8top := 0       // set to the top card of the straight, if any
	for _, ci := range c {
		ranks[ci.Rank()] += 1
		dupes[ranks[ci.Rank()]]++
		dupes[ranks[ci.Rank()]-1]--
		for i := 0; i < 5; i++ {
			idx := (int(ci.Rank()) + i) % 13
			str8s[idx] |= 1 << uint(i)
			// Make sure to exclude wrap-around straights headed by 2, 3, 4.
			if str8s[idx] == 31 && (idx <= 1 || idx >= 5) {
				str8top = (idx+12)%13 + 1
			}
		}
	}
	if !flush && str8top == 0 && dupes[1] == len(c) { // No pair
		return evalScore("%s-%s-%s-%s-%s", 0, find1(ranks, 0), find1(ranks, 1), find1(ranks, 2), find1(ranks, 3), find1(ranks, 4)), nil
	}
	if dupes[2] == 1 && dupes[3] == 0 { // One pair
		return evalScore("%[1]s%[1]s-%s-%s-%s", 1, find2(ranks, 0), find1(ranks, 0), find1(ranks, 1), find1(ranks, 2)), nil
	}
	if dupes[2] == 2 { // Two pair
		return evalScore("%[1]s%[1]s-%[2]s%[2]s-%[3]s", 2, find2(ranks, 0), find2(ranks, 1), find1(ranks, 0)), nil
	}
	if dupes[3] == 1 && dupes[2] == 0 { // Trips
		if replace {
			return evalScore("%[1]s%[1]s%[1]s-%s-%s", 3, find(3, ranks, 0), find1(ranks, 0), find1(ranks, 1)), nil
		}
		if len(c) == 5 {
			return evalScore("%[1]s%[1]s%[1]s-x-y", 3, find(3, ranks, 0)), nil // ignore kickers
		}
		return evalScore("%[1]s%[1]s%[1]s", 3, find(3, ranks, 0)), nil
	}
	if str8top != 0 && !flush { // Straight
		return evalScore("%s straight", 4, (str8top+11)%13+2), nil
	}
	if flush && str8top == 0 { // Flush
		return evalScore("%s%s%s%s%s flush", 5, find1(ranks, 0), find1(ranks, 1), find1(ranks, 2), find1(ranks, 3), find1(ranks, 4)), nil
	}
	if dupes[2] == 1 && dupes[3] == 1 { // Full house
		if replace {
			return evalScore("%[1]s%[1]s%[1]s-%[2]s%[2]s", 6, find(3, ranks, 0), find2(ranks, 0)), nil
		}
		return evalScore("%[1]s%[1]s%[1]s-xx", 6, find(3, ranks, 0)), nil // ignore lower pair
	}
	if dupes[4] == 1 { // Quads
		if replace {
			return evalScore("%[1]s%[1]s%[1]s%[1]s-%[2]s", 7, find(4, ranks, 0), find1(ranks, 0)), nil
		}
		return evalScore("%[1]s%[1]s%[1]s%[1]s-x", 7, find(4, ranks, 0)), nil // ignore kicker
	}
	if str8top != 0 && flush { // Straight flush
		return evalScore("%s straight flush", 8, (str8top+11)%13+2), nil
	}
	if dupes[5] == 1 { // 5-kind
		return evalScore("%[1]s%[1]s%[1]s%[1]s%[1]s", 9, find(5, ranks, 0)), nil
	}
	return eval{}, fmt.Errorf("failed to eval hand %v", c)
}

// ScoreMax is the largest possible result from Eval (with replace=false).
const ScoreMax = 6785

func index(c []Card) int32 {
	r := int32(1)
	suits := uint16(0x3c0)
	for _, ci := range c {
		r *= int32(ci & 0x3f)
		suits &= uint16(ci)
	}
	if len(c) == 5 && suits&0x3c0 != 0 {
		return -r
	}
	return r
}

type evalTableEntry struct {
	key int32
	val int16
}

var evalTable [32768]evalTableEntry

const evalMask = 0x7fff

var (
	rankTo5 = map[int16][]Card{}
	rankTo3 = map[int16][]Card{}
)

// EvalToHand5 returns an example 5-card hand with the given
// eval score. The second return value is whether the result is valid.
func EvalToHand5(e int16) ([]Card, bool) {
	return rankTo5[e], len(rankTo5[e]) != 0
}

// EvalToHand3 returns an example 3-card hand with the given
// eval score. The second return value is whether the result is valid.
func EvalToHand3(e int16) ([]Card, bool) {
	return rankTo3[e], len(rankTo3[e]) != 0
}

// Eval takes a 3- or 5- card poker hand and returns a number
// which can be used to rank it against other poker hands.
func Eval(c []Card) int16 {
	key := index(c)
	k := key
	for {
		if evalTable[k&evalMask].key == key {
			return evalTable[k&evalMask].val
		}
		k = hash(k)
	}
}

// Eval5 is an optimized version of Eval which requires a 5-card hand.
func Eval5(c *[5]Card) int16 {
	key := int32(c[0]&0x3f) * int32(c[1]&0x3f) * int32(c[2]&0x3f) * int32(c[3]&0x3f) * int32(c[4]&0x3f)
	if c[0]&c[1]&c[2]&c[3]&c[4]&0x3c0 != 0 {
		key = -key
	}
	k := key
	for {
		if evalTable[k&evalMask].key == key {
			return evalTable[k&evalMask].val
		}
		k = hash(k)
	}
}

// Eval3 is an optimized version of Eval which requires a 3-card hand.
func Eval3(c *[3]Card) int16 {
	key := int32(c[0]&0x3f) * int32(c[1]&0x3f) * int32(c[2]&0x3f)
	k := key
	for {
		if evalTable[k&evalMask].key == key {
			return evalTable[k&evalMask].val
		}
		k = hash(k)
	}
}

func nextIdx(ix []int, k int, dupes int) bool {
	i := 0
	for {
		ix[i]++
		if i+1 == len(ix) || ix[i] != ix[i+1]+dupes {
			return ix[i] < k
		}
		ix[i] = i * (1 - dupes)
		i++
	}
}

func hash(k int32) int32 {
	return (k >> 4) ^ (k << 6)
}

func init() {
	uniqScores := map[int]bool{}
	scores := map[int32]int{}
	hand5, hand3 := map[int][]Card{}, map[int][]Card{}
	for _, size := range []int{3, 5} {
		indexes := make([]int, size)
		hand := make([]Card, size)
		// We iterate over enough hands to categorize _all_ hands.
		// For non-flush hands we allow duplicate cards (eg: pairs)
		// but fix the suits. For flush hands, we don't allow duplicate
		// cards, and fix the suit to be spades.
		s := []Suit{Club, Diamond, Heart, Spade, Club}
		flushTop := size / 5 // 0 if size=3, 1 if size=5.
		for flush := 0; flush <= flushTop; flush++ {
			if flush == 1 {
				for i := range indexes {
					indexes[i] = i
				}
			}
			for {
				for i, ix := range indexes {
					suit := Spade
					if flush == 0 {
						suit = s[i]
					}
					var err error
					hand[i], err = MakeCard(suit, Rank(ix+1))
					if err != nil {
						log.Fatalf("failed to create card: %s", err)
					}
				}
				idx := index(hand)
				ev, err := evalSlow(hand, false)
				if err != nil {
					log.Fatalf("evalSlow(%v) gave error %s", hand, err)
				}
				if oldEV, ok := scores[idx]; ok && oldEV != ev.rank {
					log.Fatalf("found different evals for hand %v", hand)
				}
				scores[idx] = ev.rank
				if size == 3 {
					hand3[ev.rank] = append([]Card{}, hand...)
				} else {
					hand5[ev.rank] = append([]Card{}, hand...)
				}
				uniqScores[ev.rank] = true
				if !nextIdx(indexes, 13, 1-flush) {
					break
				}
			}
		}
	}
	// Aggregate and pack scores.
	allScores := []int{}
	for k := range uniqScores {
		allScores = append(allScores, k)
	}
	sort.Ints(allScores)
	keys := []int{}
	for k, _ := range scores {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		key := int32(k)
		for evalTable[key&evalMask].key != 0 {
			key = hash(key)
		}
		val := int16(sort.SearchInts(allScores, scores[int32(k)]))
		evalTable[key&evalMask] = evalTableEntry{int32(k), val}
		rankTo5[val] = hand5[scores[int32(k)]]
		rankTo3[val] = hand3[scores[int32(k)]]
	}
	if ScoreMax != len(allScores)-1 {
		log.Fatalf("Expected max score of %d, but found %d", ScoreMax, len(allScores)-1)
	}
}

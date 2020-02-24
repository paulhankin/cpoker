package cpoker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"

	"github.com/paulhankin/poker/v2/poker"
)

// A RolloutEvaluator gives the opponent random hands, sees
// what they play for each, then plays the current hand to
// maximize the outcome.
type RolloutEvaluator struct {
	PreRollout bool
	Separable  bool // score hand by treating f/m/b as independent.
	Opponent   HandEvaluator
	N          int // how many rollouts we do
	played     [][3]int16
	wins       [3][]float64
}

// A SampledEvaluator evaluates hands based on independent probabilities the
// front, middle, and back hands will win.
type SampledEvaluator struct {
	wins [3][]float64
}

// WinProbabilities returns a mapping from rank (from Eval) to
// a probability that the hand wins. i=0,1,2 means front,middle,back.
func (se *SampledEvaluator) WinProbabilities(i int) []float64 {
	if i < 0 || i > 2 {
		return nil
	}
	return se.wins[i]
}

// NewSampledEvaluatorFromRollout converts a separable, pre-rolled out
// RolloutEvaluator into a SampledEvaluator. The RolloutEvaluator must
// have already have sampled hands.
func NewSampledEvaluatorFromRollout(re *RolloutEvaluator) (*SampledEvaluator, error) {
	if !re.Separable {
		return nil, errors.New("rollout evaluator not separable")
	}
	if !re.PreRollout {
		return nil, errors.New("rollout evaluator not pre-rolled-out")
	}
	if len(re.wins) == 0 {
		return nil, errors.New("rollout evaluator hasn't been prepared")
	}
	return &SampledEvaluator{
		wins: [3][]float64{
			append([]float64{}, re.wins[0]...),
			append([]float64{}, re.wins[1]...),
			append([]float64{}, re.wins[2]...),
		},
	}, nil
}

// Evaluator returns a hand evaluator for the given set of cards.
func (se *SampledEvaluator) Evaluator(cs []poker.Card) func(f, m, b int16) float64 {
	return se.evaluateHand
}

// evaluateHand returns an expected value for playing a hand with
// the given ranks for the front, middle, and back hands.
func (se *SampledEvaluator) evaluateHand(f, m, b int16) float64 {
	pf := se.wins[0][f]
	pm := se.wins[1][m]
	pb := se.wins[2][b]
	qf := 1 - pf
	qm := 1 - pm
	qb := 1 - pb
	pbon := pf*pm + pf*pb + pm*pb - 2*pf*pm*pb
	qbon := qf*qm + qf*qb + qm*qb - 2*qf*qm*qb
	return pf + pm + pb - qf - qm - qb + pbon - qbon
}

// NewTrainedSampledEvaluator constructs a SampledEvaluator based
// on a sampling of the given opponent evaluator (with N samples).
// If the opponent is itself a SampledEvaluator or a suitable RolloutEvaluator
// then the win probabilities are averaged between the opponent and
// the exploiting probabilities found.
func NewTrainedSampledEvaluator(opp HandEvaluator, N int) *SampledEvaluator {
	e := &RolloutEvaluator{PreRollout: true, Separable: true, Opponent: opp, N: N}
	e.Init()
	var oppWins *[3][]float64
	if se, ok := opp.(*SampledEvaluator); ok {
		oppWins = &se.wins
	}
	if re, ok := opp.(*RolloutEvaluator); ok && re.PreRollout && re.Separable && len(re.wins) > 0 {
		oppWins = &re.wins
	}
	if oppWins != nil {
		for i := 0; i < 3; i++ {
			for j := range (*oppWins)[i] {
				e.wins[i][j] = (e.wins[i][j] + (*oppWins)[i][j]) / 2
			}
		}
	}
	r, err := NewSampledEvaluatorFromRollout(e)
	if err != nil {
		log.Fatalf("internal error: %s", err)
	}
	return r
}

// Marshal writes a SampledEvaluator to the given file.
func (se *SampledEvaluator) Marshal(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for i := 0; i < 3; i++ {
		fmt.Fprintf(bw, "%d ", len(se.wins[i]))
		for _, c := range se.wins[i] {
			fmt.Fprintf(bw, "%f ", c)
		}
	}
	return bw.Flush()
}

// Save writes a SampledEvaluator to a named file.
func (se *SampledEvaluator) Save(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err := se.Marshal(f); err != nil {
		return err
	}
	return f.Close()
}

// LoadSampledEvaluator reads a SampledEvaluator from a named file.
func LoadSampledEvaluator(filename string) (*SampledEvaluator, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return UnmarshalSampledEvaluator(f)
}

// UnmarshalSampledEvaluator reads weights from the given
// file, constructing a SampledEvaluator.
func UnmarshalSampledEvaluator(r io.Reader) (*SampledEvaluator, error) {
	se := SampledEvaluator{}
	for i := 0; i < 3; i++ {
		length := 0
		if _, err := fmt.Fscanf(r, "%d", &length); err != nil {
			return nil, err
		}
		se.wins[i] = make([]float64, length)
		for j := range se.wins[i] {
			if _, err := fmt.Fscanf(r, "%f", &se.wins[i][j]); err != nil {
				return nil, err
			}
		}
	}
	return &se, nil
}

func rollout(cs []poker.Card, opp HandEvaluator, N int) (played [][3]int16, wins [3][]float64) {
	deck := make([]poker.Card, 0, 52-len(cs))
	h := map[poker.Card]bool{}
	for _, c := range cs {
		h[c] = true
	}
	for _, c := range poker.Cards {
		if !h[c] {
			deck = append(deck, c)
		}
	}
	played = make([][3]int16, N)
	cases := make(chan int, 16)
	workers := 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			mydeck := append([]poker.Card{}, deck...)
			for c := range cases {
				for i := 0; i < 13; i++ {
					j := rand.Intn(len(mydeck)-i) + i
					mydeck[i], mydeck[j] = mydeck[j], mydeck[i]
				}
				hand, _ := Play(mydeck[:13], opp)
				played[c] = [3]int16{
					poker.Eval3(&hand.Front), poker.Eval5(&hand.Middle), poker.Eval5(&hand.Back),
				}
			}
			wg.Done()
		}()
	}
	for i := range played {
		cases <- i
	}
	close(cases)
	wg.Wait()
	for i := 0; i < 3; i++ {
		wins[i] = make([]float64, poker.ScoreMax+1)
	}
	for _, s := range played {
		for i := 0; i < 3; i++ {
			wins[i][s[i]]++
		}
	}
	for i := 0; i < 3; i++ {
		t := 0.0
		for j := range wins[i] {
			t += wins[i][j]
			wins[i][j] = t / float64(N)
		}
	}
	return played, wins
}

// Init pre-rolls-out the rollout evaluator if necessary.
func (re *RolloutEvaluator) Init() {
	if !re.PreRollout {
		return
	}
	re.played, re.wins = rollout(nil, re.Opponent, re.N)
}

// Evaluator returns a hand evaluator for the given set of cards. Depending
// on the options, this may or may not involve performing an expensive
// rollout first.
func (re *RolloutEvaluator) Evaluator(cs []poker.Card) func(f, m, b int16) float64 {
	played, wins := re.played, re.wins
	if !re.PreRollout {
		played, wins = rollout(cs, re.Opponent, re.N)
	}
	if re.Separable {
		se := &SampledEvaluator{wins}
		return se.Evaluator(nil)
	}
	return func(f, m, b int16) float64 {
		score := 0
		for _, p := range played {
			score += cmp(f, p[0], m, p[1], b, p[2])
		}
		return float64(score) + float64(f+m+b)/10000.0
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cmp takes interleaved ranks of three hands, and returns a score for the
// first player.
func cmp(a0, b0, a1, b1, a2, b2 int16) int {
	wins := b2i(a0 > b0) + b2i(a1 > b1) + b2i(a2 > b2)
	losses := b2i(b0 > a0) + b2i(b1 > a1) + b2i(b2 > a2)
	bonus := b2i(wins > losses) - b2i(losses > wins)
	return wins - losses + bonus
}

// CompareHands returns a score for player 0, assuming player 0 plays h0 and
// player 1 plays h1. The function assumes both hands are legal.
// The scoring used is 2-4 scoring: one point for each place won, and one point
// for winning the majority of the places.
func CompareHands(h0, h1 *Hand) int {
	return cmp(poker.Eval3(&h0.Front), poker.Eval3(&h1.Front), poker.Eval5(&h0.Middle), poker.Eval5(&h1.Middle), poker.Eval5(&h0.Back), poker.Eval5(&h1.Back))
}

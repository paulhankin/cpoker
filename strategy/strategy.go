// Binary strategy outputs tables showing hand-values
// for a chinese poker hand.
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/paulhankin/cpoker"
	"github.com/paulhankin/poker"
)

var (
	fromFile = flag.String("from", "", "file to load coefficients from")
	mode     = flag.String("mode", "rank", "all/ends/percent/per5 : show all hands, just the end of each range, or one hand per percent, one hand per 5 percent")
)

var ends5m = [][2]string{
	{"75432", "AKQJ9"},
	{"22345", "TTAKQ"},
	{"JJ432", "JJAKQ"},
	{"QQ432", "QQAKJ"},
	{"KK432", "KKAQJ"},
	{"AA432", "AAKQJ"},
	{"33224", "8877A"},
	{"99223", "TT99A"},
	{"JJ223", "JJTTA"},
	{"QQ223", "QQJJA"},
	{"KK223", "KKQQA"},
	{"AA223", "AAKKQ"},
	{"22234", "AAAKQ"},
	{"A2345", "87654"},
	{"98765", "AKQJT"},
	{"76542s", "AKQJ9s"},
	{"22233", "AAAKK"},
	{"22223", "AAAAK"},
	{"A2345s", "TJQKAs"},
}

var ends5b = [][2]string{
	{"75432", "AKQJ9"},
	{"22345", "AAKQJ"},
	{"33224", "AAKKQ"},
	{"22234", "AAAKQ"},
	{"A2345", "87654"},
	{"98765", "AKQJT"},
	{"75432s", "T9875s"},
	{"J5432s", "JT986s"},
	{"Q5432s", "QJT97s"},
	{"K5432s", "KJT97s"},
	{"KQ432s", "KQJT8s"},
	{"A6432s", "AJT98s"},
	{"AQ432s", "AQJT9s"},
	{"AK432s", "AKQJ9s"},
	{"22233", "66633"},
	{"77722", "TTT22"},
	{"JJJ22", "AAA22"},
	{"22223", "AAAAK"},
	{"A2345s", "TJQKAs"},
}

var ends3 = [][2]string{
	{"432", "987"},
	{"T32", "QJT"},
	{"K32", "KQJ"},
	{"A32", "AT9"},
	{"AJ2", "AJT"},
	{"AQ2", "AQJ"},
	{"AK2", "AKQ"},
	{"223", "66A"},
	{"772", "77A"},
	{"882", "88A"},
	{"992", "99A"},
	{"TT2", "TTA"},
	{"JJ2", "JJA"},
	{"QQ2", "QQA"},
	{"KK2", "KKA"},
	{"AA2", "AAK"},
	{"222", "AAA"},
}

func atoc(s poker.Suit, rank rune) poker.Card {
	ranks := "-A23456789TJQK"
	for i, r := range ranks {
		if r == rank {
			result, err := poker.MakeCard(s, poker.Rank(i))
			if err != nil {
				log.Fatalf("MakeCard(%v, %v) failed: %s", s, r, err)
			}
			return result
		}
	}
	log.Fatalf("failed to parse card %v %v", s, rank)
	return 0
}

func parseHand(s string) []poker.Card {
	flush := false
	suits := []poker.Suit{poker.Club, poker.Diamond, poker.Spade, poker.Heart, poker.Club}
	if s[len(s)-1] == 's' {
		flush = true
		s = s[:len(s)-1]
	}
	result := make([]poker.Card, 0, len(s))
	for i, c := range s {
		suit := suits[i]
		if flush {
			suit = poker.Heart
		}
		result = append(result, atoc(suit, c))
	}
	return result
}

func mustDescribeShort(c []poker.Card) string {
	r, err := poker.DescribeShort(c)
	if err != nil {
		log.Fatalf("failed to describe %s: %s", c, err)
	}
	return r
}

func ends(se *cpoker.SampledEvaluator) {
	parts := []string{"front", "middle", "back"}
	fmt.Printf("|            |%-60s| __Winning Percentage__ |\n", " __Hand Range__")
	fmt.Printf("|------------|%-60s|:-----------------------|\n", ":"+strings.Repeat("-", 58)+":")
	for i := range parts {
		fmt.Printf("| %-10s |%60s|%24s|\n", "__"+parts[i]+"__", "", "")
		ends := [][][2]string{ends3, ends5m, ends5b}[i]
		wins := se.WinProbabilities(i)
		for _, es := range ends {
			h0 := parseHand(es[0])
			h1 := parseHand(es[1])
			d0 := mustDescribeShort(h0)
			d1 := mustDescribeShort(h1)
			fmt.Printf("|%12s| %21s &mdash; %-21s &nbsp; | %6.2f &mdash; %6.2f  |\n", "", d0, d1, wins[poker.Eval(h0)]*100, wins[poker.Eval(h1)]*100)
		}
	}
	fmt.Println()
}

func percents(se *cpoker.SampledEvaluator, x float64) {
	parts := []string{"front", "middle", "back"}
	for i := range parts {
		wantLen := 3
		fmt.Println(parts[i])
		toHand := poker.EvalToHand3
		if i > 0 {
			toHand = poker.EvalToHand5
			wantLen = 5
		}
		oldp := 0.0
		last := ""
		for r, p := range se.WinProbabilities(i) {
			if x != 0 && int(p*x) == int(oldp*x) {
				continue
			}
			h, ok := toHand(int16(r))
			if !ok || len(h) != wantLen {
				continue
			}
			// We can have multiple hands with the same short description.
			// For example AAA-22, AAA-33, ..., AAA-KK.
			// We show only the first that appears in the output.
			// This isn't quite right because the really the output
			// should be an average, but the differences are tiny.
			rShort := mustDescribeShort(h)
			if rShort != last {
				fmt.Printf("%5.2f : %s\n", 100*p, mustDescribeShort(h))
				last = rShort
			}
			oldp = p
		}
		fmt.Println("")
	}
}

func main() {
	flag.Parse()
	if *fromFile == "" {
		log.Fatalf("-from must be specified")
	}
	se, err := cpoker.LoadSampledEvaluator(*fromFile)
	if err != nil {
		log.Fatalf("failed to load coefficients: %s", err)
	}
	switch *mode {
	case "percent":
		percents(se, 100)
	case "all":
		percents(se, 0)
	case "per5":
		percents(se, 20)
	case "ends":
		ends(se)
	default:
		log.Fatalf("Unknown value for flag -mode: <%s>", *mode)
	}
}

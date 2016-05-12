// Binary train trains and tests chinese poker evaluators.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/paulhankin/cpoker"
)

var (
	fromFile       = flag.String("from", "", "file to read weights from")
	toFile         = flag.String("to", "", "file to write trained weights to")
	trainN         = flag.Int("hands", 0, "how many hands to train on")
	trainCycles    = flag.Int("train_cycles", 1, "how many training iterations to perform")
	evalSamples    = flag.Int("eval_samples", 10000, "how many hands to use to produce the optimal opponent")
	evalHands      = flag.Int("eval_hands", 0, "how many hands to evaluate the trained evaluator on")
	evalSep        = flag.Bool("eval_separable", true, "consider front/middle/back as independent when training the opponent")
	evalRollAll    = flag.Bool("eval_rollall", false, "rollout every hand separately")
	evalPrintEvery = flag.Int("eval_printn", 100, "show running summaries for eval every this many hands")
)

func main() {
	flag.Parse()
	if *toFile == "" && *evalHands == 0 {
		log.Fatalln("the trained evaluator must be written to a file (with -to) or evaluated (with -eval_hands)")
	}
	if *evalHands > 0 && *evalSamples <= 0 {
		log.Fatalln("eval_samples must be positive if an evaluation is asked for")
	}
	var hero cpoker.HandEvaluator = cpoker.MaxProdEvaluator{} // Default is simple rank-based evaluator.
	if *fromFile != "" {
		var err error
		if hero, err = cpoker.LoadSampledEvaluator(*fromFile); err != nil {
			log.Fatalf("failed to load evaluator: %s", err)
		}
	}
	if *trainN > 0 {
		for i := 0; i < *trainCycles; i++ {
			log.Printf("Training cycle: %d/%d\n", i+1, *trainCycles)
			hero = cpoker.NewTrainedSampledEvaluator(hero, *trainN)
		}
	}
	if *toFile != "" {
		se, ok := hero.(*cpoker.SampledEvaluator)
		if !ok {
			log.Fatalln("can't save initial evaluator")
		}
		if err := se.Save(*toFile); err != nil {
			log.Fatalf("failed to save evaluator: %s", err)
		}
	}
	if *evalHands == 0 {
		return
	}
	opp := &cpoker.RolloutEvaluator{PreRollout: !*evalRollAll, Separable: *evalSep, Opponent: hero, N: *evalSamples}
	log.Println("training optimal opponent...")
	opp.Init()
	log.Println("running comparison...")
	fmt.Printf("\n%+v", cpoker.CompareEvaluators(hero, opp, *evalHands, *evalPrintEvery))
}

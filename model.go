package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dmarkham/goNEAT/experiments"
	"github.com/dmarkham/goNEAT/neat"
	"github.com/dmarkham/goNEAT/neat/genetics"
	"github.com/hajimehoshi/ebiten"
)

type ModelEvaluator struct {
	sync.Mutex
	OutputPath string
	g          *Game
	tick       uint64
}

func (ex *ModelEvaluator) Draw(screen *ebiten.Image) error {

	ex.Lock()
	defer ex.Unlock()
	if ex.g == nil {
		return nil
	}

	ex.g.Draw(screen)
	ex.tick = ex.g.steps
	return nil
}

func (ex *ModelEvaluator) GenerationEvaluate(pop *genetics.Population, epoch *experiments.Generation, context *neat.NeatContext) error {
	ex.Lock()
	g := NewGame(pop)
	ex.g = g
	ex.Unlock()

	for {

		time.Sleep(time.Millisecond * 14)
		ex.Lock()
		err := g.Update()
		ex.Unlock()
		if err != nil {
			break
		}

	}
	// Evaluate each organism on a test
	b := 0.0
	for _, org := range pop.Organisms {
		if org.Fitness > b {
			b = org.Fitness
		}
		winner := org.IsWinner
		if winner && (epoch.Best == nil || org.Fitness > epoch.Best.Fitness) {
			epoch.Solved = true
			epoch.WinnerNodes = len(org.Genotype.Nodes)
			epoch.WinnerGenes = org.Genotype.Extrons()
			epoch.WinnerEvals = context.PopSize*epoch.Id + org.Genotype.Id
			epoch.Best = org
			org.IsWinner = true
		}

	}

	fmt.Printf("Generation: %v  Best: %.06f Last Pop %.06f\n", epoch.Id, pop.HighestFitness, b)
	epoch.FillPopulationStatistics(pop)

	//fmt.Println("Best: ", pop.HighestFitness)
	// Only print to file every print_every generations
	if epoch.Solved || epoch.Id%context.PrintEvery == 0 {
		pop_path := fmt.Sprintf("%s/gen_%d", experiments.OutDirForTrial(ex.OutputPath, epoch.TrialId), epoch.Id)
		file, err := os.Create(pop_path)

		if err != nil {
			neat.ErrorLog(fmt.Sprintf("Failed to dump population, reason: %s\n", err))
		} else {
			pop.WriteBySpecies(file)
		}
	}

	if b > .30 {
		org := epoch.Best
		orgPath := fmt.Sprintf("%s/%s_%.8f_%d-%d", experiments.OutDirForTrial(ex.OutputPath, epoch.TrialId),
			"best", org.Fitness, org.Phenotype.NodeCount(), org.Phenotype.LinkCount())
		file, err := os.Create(orgPath)
		if err != nil {
			neat.ErrorLog(fmt.Sprintf("Failed to dump best organism genome, reason: %s\n", err))
		} else {
			org.Genotype.Write(file)
			neat.InfoLog(fmt.Sprintf("Generation #%d best %d dumped to: %s\n", epoch.Id, org.Genotype.Id, orgPath))
		}
		file.Close()

	}
	if epoch.Solved {
		// print winner organism
		for _, org := range pop.Organisms {
			if org.IsWinner {
				// Prints the winner organism to file!
				orgPath := fmt.Sprintf("%s/%s_%.1f_%d-%d", experiments.OutDirForTrial(ex.OutputPath, epoch.TrialId),
					"winner", org.Fitness, org.Phenotype.NodeCount(), org.Phenotype.LinkCount())
				file, err := os.Create(orgPath)
				if err != nil {
					neat.ErrorLog(fmt.Sprintf("Failed to dump winner organism genome, reason: %s\n", err))
				} else {
					org.Genotype.Write(file)
					neat.InfoLog(fmt.Sprintf("Generation #%d winner %d dumped to: %s\n", epoch.Id, org.Genotype.Id, orgPath))
				}
				break
			}
		}
		return nil
	}

	return nil
}

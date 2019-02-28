package main

import (
	"fmt"
	"os"

	"github.com/yaricom/goNEAT/experiments"
	"github.com/yaricom/goNEAT/neat"
	"github.com/yaricom/goNEAT/neat/genetics"
)

type ModelEvaluator struct {
	OutputPath string
}

func (ex ModelEvaluator) GenerationEvaluate(pop *genetics.Population, epoch *experiments.Generation, context *neat.NeatContext) error {

	g := NewGame(pop)
	for {
		err := g.Update()
		if err != nil {
			break
		}
	}
	// Evaluate each organism on a test
	b := 0.0
	for _, org := range pop.Organisms {
		//winner, err := ex.orgEvaluate(org)
		//if err != nil {
		//	return err
		//}
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

	fmt.Printf("Generation: %v  Best: %.02f Last Pop %.02f\n", epoch.Id, pop.HighestFitness, b)
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

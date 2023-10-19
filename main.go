package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/dmarkham/goNEAT/experiments"
	"github.com/dmarkham/goNEAT/neat"
	"github.com/dmarkham/goNEAT/neat/genetics"
	"github.com/hajimehoshi/ebiten"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	conf := viper.New()
	pflag.String("config", "config.neat", "Config for Neat")
	pflag.String("out-dir", "/tmp/out", "Default output Dir")
	pflag.Bool("profile", false, "turn Profile on")
	pflag.Parse()
	conf.BindPFlags(pflag.CommandLine)
	profile := conf.GetBool("profile")

	configFile, err := os.Open(conf.GetString("config"))
	if err != nil {
		log.Fatal("Failed to open context configuration file: ", err)
	}
	context := neat.LoadContext(configFile)

	fmt.Println(context)
	// Check if output dir exists
	outDir := conf.GetString("out-dir")
	if _, err := os.Stat(outDir); err == nil {
		// backup it
		backupDir := fmt.Sprintf("%s-%s", outDir, time.Now().Format("2006-01-02T15_04_05"))
		// clear it
		err = os.Rename(outDir, backupDir)
		if err != nil {
			log.Fatal("Failed to do previous results backup: ", err)
		}
	}
	// create output dir
	err = os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		log.Fatal("Failed to create output directory: ", err)
	}

	if profile {
		f, err := os.Create("cpuprofile")
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	experiment := experiments.Experiment{
		Id:     0,
		Trials: make(experiments.Trials, context.NumRuns),
	}
	// This special constructor creates a Genome with in inputs, out outputs, n out of nmax hidden units, and random
	// connectivity.  If rec is true then recurrent connections will be included. The last input is a bias
	// link_prob is the probability of a link. The created genome is not modular.
	start_genome := genetics.NewGenomeRand(1, 5, 1, 1, 10, false, .7)
	fmt.Printf(">>> Start genome file:  %s\n", start_genome.String())
	//pop, err := genetics.NewPopulationRandom(6, 1, 14, false, .75, context)
	//if err != nil {
	//	panic(err)
	//}
	//ok, err := pop.Verify()
	//if !ok {
	//	panic(err)
	//}
	var _ experiments.GenerationEvaluator = (*ModelEvaluator)(nil)
	mEval := &ModelEvaluator{OutputPath: outDir}

	ebiten.SetRunnableInBackground(true)
	ebiten.SetMaxTPS(10)

	go func() {
		err = experiment.Execute(context, start_genome, mEval)
		if err != nil {
			log.Fatal("Failed to perform XOR experiment: ", err)
		}
		if profile {
			f, err := os.Create("memprofile")
			if err != nil {
				log.Fatal("could not create memory profile: ", err)
			}
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
			f.Close()
		}
		// Print statistics
		experiment.PrintStatistics()

		// Save experment data
		expResPath := fmt.Sprintf("%s/%s.dat", outDir, "MyExp")
		expResFile, err := os.Create(expResPath)
		if err == nil {
			err = experiment.Write(expResFile)
		}
		if err != nil {
			log.Fatal("Failed to save experiment results", err)
		}

	}()
	ebiten.Run(mEval.Draw, screenWidth, screenHeight, 1, "Flappy Gopher (Ebiten Demo)")
}

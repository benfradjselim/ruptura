package cmd

import (
	"fmt"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/spf13/cobra"
)

var (
	wSelector  string
	wStress    float64
	wFatigue   float64
	wMood      float64
	wPressure  float64
	wHumidity  float64
	wContagion float64
)

var weightsCmd = &cobra.Command{
	Use:   "weights",
	Short: "Manage per-workload signal weights",
	Example: `  ruptura-ctl weights get
  ruptura-ctl weights set --selector "production/*" --stress 0.4 --fatigue 0.25
  ruptura-ctl weights set --selector "*" --stress 0.3 --fatigue 0.2 --mood 0.2 --pressure 0.15 --humidity 0.1 --contagion 0.05`,
	RunE: weightsGetCmd.RunE,
}

var weightsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current per-workload signal weight configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := newClient()
		cfgs, err := c.Weights(ctx())
		if err != nil {
			return fmt.Errorf("fetch weights: %w", err)
		}
		if cfgOutput == "json" {
			return printJSON(cfgs)
		}
		if len(cfgs) == 0 {
			fmt.Println(dim("\n  No custom weights configured — using defaults."))
			fmt.Printf("  %s\n\n", dim("Default: stress=0.25 fatigue=0.20 mood=0.20 pressure=0.15 humidity=0.10 contagion=0.10"))
			return nil
		}
		t := newTable("SELECTOR", "STRESS", "FATIGUE", "MOOD", "PRESSURE", "HUMIDITY", "CONTAGION")
		t.alignRight(1)
		t.alignRight(2)
		t.alignRight(3)
		t.alignRight(4)
		t.alignRight(5)
		t.alignRight(6)
		for _, w := range cfgs {
			t.add(
				cyan(w.Selector),
				fmt.Sprintf("%.2f", w.Stress),
				fmt.Sprintf("%.2f", w.Fatigue),
				fmt.Sprintf("%.2f", w.Mood),
				fmt.Sprintf("%.2f", w.Pressure),
				fmt.Sprintf("%.2f", w.Humidity),
				fmt.Sprintf("%.2f", w.Contagion),
			)
		}
		t.print()
		return nil
	},
}

var weightsSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Add or replace a signal weight config",
	Long: `Add or replace a per-workload signal weight configuration.

Selector syntax:
  exact    production/Deployment/payment-api
  prefix   production/*     (all workloads in namespace)
  global   *                (all workloads)

Weights are automatically normalised to sum 1.0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if wSelector == "" {
			return fmt.Errorf("--selector is required")
		}
		total := wStress + wFatigue + wMood + wPressure + wHumidity + wContagion
		if total == 0 {
			return fmt.Errorf("at least one weight flag must be set")
		}

		c := newClient()
		existing, err := c.Weights(ctx())
		if err != nil {
			return fmt.Errorf("fetch existing weights: %w", err)
		}

		// replace or append
		found := false
		for i, w := range existing {
			if w.Selector == wSelector {
				existing[i] = buildWeights()
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, buildWeights())
		}

		if err := c.SetWeights(ctx(), existing); err != nil {
			return fmt.Errorf("set weights: %w", err)
		}

		// Display normalised values (what the server actually uses)
		norm := func(v float64) float64 {
			if total == 0 {
				return 0
			}
			return v / total
		}
		successLine(fmt.Sprintf("Weights applied for selector %s", cyan(wSelector)))
		fmt.Printf("  Normalised → stress=%.2f  fatigue=%.2f  mood=%.2f  pressure=%.2f  humidity=%.2f  contagion=%.2f\n\n",
			norm(wStress), norm(wFatigue), norm(wMood), norm(wPressure), norm(wHumidity), norm(wContagion))
		return nil
	},
}

var weightsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove weight config for a selector",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wSelector == "" {
			return fmt.Errorf("--selector is required")
		}
		c := newClient()
		existing, err := c.Weights(ctx())
		if err != nil {
			return err
		}
		filtered := existing[:0]
		for _, w := range existing {
			if w.Selector != wSelector {
				filtered = append(filtered, w)
			}
		}
		if len(filtered) == len(existing) {
			return fmt.Errorf("no weight config found for selector %q", wSelector)
		}
		if err := c.SetWeights(ctx(), filtered); err != nil {
			return fmt.Errorf("update weights: %w", err)
		}
		successLine(fmt.Sprintf("Weight config removed for %s", cyan(wSelector)))
		fmt.Println()
		return nil
	},
}

func buildWeights() models.SignalWeights {
	return models.SignalWeights{
		Selector:  wSelector,
		Stress:    wStress,
		Fatigue:   wFatigue,
		Mood:      wMood,
		Pressure:  wPressure,
		Humidity:  wHumidity,
		Contagion: wContagion,
	}
}

func init() {
	weightsCmd.AddCommand(weightsGetCmd)
	weightsCmd.AddCommand(weightsSetCmd)
	weightsCmd.AddCommand(weightsDeleteCmd)
	rootCmd.AddCommand(weightsCmd)

	for _, c := range []*cobra.Command{weightsSetCmd, weightsDeleteCmd} {
		c.Flags().StringVar(&wSelector, "selector", "", "Workload selector: exact, namespace/*, or *")
	}
	weightsSetCmd.Flags().Float64Var(&wStress, "stress", 0, "Stress weight (0.0–1.0)")
	weightsSetCmd.Flags().Float64Var(&wFatigue, "fatigue", 0, "Fatigue weight (0.0–1.0)")
	weightsSetCmd.Flags().Float64Var(&wMood, "mood", 0, "Mood weight (0.0–1.0)")
	weightsSetCmd.Flags().Float64Var(&wPressure, "pressure", 0, "Pressure weight (0.0–1.0)")
	weightsSetCmd.Flags().Float64Var(&wHumidity, "humidity", 0, "Humidity weight (0.0–1.0)")
	weightsSetCmd.Flags().Float64Var(&wContagion, "contagion", 0, "Contagion weight (0.0–1.0)")
}

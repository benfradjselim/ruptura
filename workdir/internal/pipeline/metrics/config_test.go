package metrics

import "testing"

func TestDefaultEngineConfig(t *testing.T) {
    cfg := DefaultEngineConfig()
    if cfg.EnsembleMode != EnsembleModeAdaptive {
        t.Errorf("want adaptive, got %d", cfg.EnsembleMode)
    }
}

func TestNewEngineWithConfig_Fixed(t *testing.T) {
    cfg := EngineConfig{EnsembleMode: EnsembleModeFixed}
    e := NewEngineWithConfig(cfg)
    if e.EnsembleMode() != EnsembleModeFixed {
        t.Error("want fixed mode")
    }
}

func TestFixedWeights_Sum(t *testing.T) {
    sum := FixedWeightILR + FixedWeightHW + FixedWeightARIM
    if sum < 0.999 || sum > 1.001 {
        t.Errorf("fixed weights must sum to 1.0, got %.4f", sum)
    }
}

func TestWeights_FixedMode(t *testing.T) {
    s := newSeriesEnsemble()
    wILR, wHW, wAR := s.weights(EnsembleModeFixed)
    if wILR != FixedWeightILR || wHW != FixedWeightHW || wAR != FixedWeightARIM {
        t.Errorf("fixed weights wrong: %.2f %.2f %.2f", wILR, wHW, wAR)
    }
}

func TestWeights_AdaptiveMode_ColdStart(t *testing.T) {
    s := newSeriesEnsemble()
    wILR, wHW, wAR := s.weights(EnsembleModeAdaptive)
    // cold start: ILR gets full weight
    if wILR != 1.0 || wHW != 0.0 || wAR != 0.0 {
        t.Errorf("cold start adaptive weights wrong: %.2f %.2f %.2f", wILR, wHW, wAR)
    }
}

func TestNewEngine_BackwardsCompat(t *testing.T) {
    e := NewEngine()
    if e.EnsembleMode() != EnsembleModeAdaptive {
        t.Error("NewEngine() must default to adaptive")
    }
}

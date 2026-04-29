package metrics

// EnsembleMode controls how the ensemble combines model forecasts.
type EnsembleMode int

const (
    // EnsembleModeAdaptive uses inverse-MSE EWMA weights (default).
    EnsembleModeAdaptive EnsembleMode = iota
    // EnsembleModeFixed uses hardcoded weights for reproducibility.
    EnsembleModeFixed
)

// Fixed weights used when EnsembleModeFixed is active.
const (
    FixedWeightILR  = 0.40
    FixedWeightHW   = 0.35
    FixedWeightARIM = 0.25
)

// EngineConfig holds tunable parameters for the metrics Engine.
type EngineConfig struct {
    // EnsembleMode selects adaptive (default) or fixed-weight ensemble.
    EnsembleMode EnsembleMode
    AnomalyStoreCapacity int
}

// DefaultEngineConfig returns production defaults.
func DefaultEngineConfig() EngineConfig {
    return EngineConfig{EnsembleMode: EnsembleModeAdaptive, AnomalyStoreCapacity: 1000}
}

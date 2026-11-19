package failuremodes

// ModeMemoryBeyondUsual is a pod with no memory limit projected to grow past
// the usual level of its workload: a namespace default set by the operator, or
// the 7-day 99th percentile plus a margin. Nothing kills the pod there, so it
// is reported as unusual growth, never as an OOM kill.
const ModeMemoryBeyondUsual Mode = "memory_beyond_usual"

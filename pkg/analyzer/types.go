package analyzer

import "github.com/llm-inferno/queue-analysis/pkg/queue"

// small disturbance around a value
const Epsilon = float32(0.001)

// fraction of maximum server throughput to provide stability (running this fraction below the maximum)
const StabilitySafetyFraction = float32(0.1)

// Analyzer of inference server queue
type QueueAnalyzer struct {
	MaxBatchSize int                           // maximum batch size
	MaxQueueSize int                           // maximum queue size
	ServiceParms *ServiceParms                 // request processing parameters
	RequestSize  *RequestSize                  // number of input and output tokens per request
	Model        *queue.MM1ModelStateDependent // queueing model
	RateRange    *RateRange                    // range of request rates for model stability
}

// queue configuration parameters
type Configuration struct {
	MaxBatchSize int           // maximum batch size (limit on the number of requests concurrently receiving service >0)
	MaxQueueSize int           // maximum queue size (limit on the number of requests queued for servive >=0)
	ServiceParms *ServiceParms // request processing parameters
}

// request processing parameters
type ServiceParms struct {
	Prefill *PrefillParms // parameters to calculate prefill time
	Decode  *DecodeParms  // parameters to calculate decode time
}

// prefill time = gamma + delta * inputTokens * batchSize (msec); inputTokens > 0
type PrefillParms struct {
	Gamma float32 // base
	Delta float32 // slope
}

// decode time = alpha + beta * batchSize (msec); batchSize > 0
type DecodeParms struct {
	Alpha float32 // base
	Beta  float32 // slope
}

// request tokens data
type RequestSize struct {
	AvgInputTokens  int // average number of input tokens per request
	AvgOutputTokens int // average number of output tokens per request
}

// range of request rates (requests/sec)
type RateRange struct {
	Min float32 // lowest rate (slightly larger than zero)
	Max float32 // highest rate (slightly less than maximum service rate)
}

// analysis solution metrics data
type AnalysisMetrics struct {
	Throughput     float32 // effective throughput (requests/sec)
	AvgRespTime    float32 // average request response time (aka latency) (msec)
	AvgWaitTime    float32 // average request queueing time (msec)
	AvgNumInServ   float32 // average number of requests in service
	AvgPrefillTime float32 // average request prefill time (msec)
	AvgTokenTime   float32 // average token decode time (msec)
	MaxRate        float32 // maximum throughput (requests/sec)
	Rho            float32 // utilization
}

// queue performance targets
type TargetPerf struct {
	TargetTTFT float32 // target time to first token (queueing + prefill) (msec)
	TargetITL  float32 // target inter-token latency (msec)
	TargetTPS  float32 // target token generation throughtput (tokens/sec)
}

// queue max request rates to achieve performance targets
type TargetRate struct {
	RateTargetTTFT float32 // max request rate for target TTFT (requests/sec)
	RateTargetITL  float32 // max request rate for target ITL (requests/sec)
	RateTargetTPS  float32 // max request rate for target TPS (requests/sec)
}

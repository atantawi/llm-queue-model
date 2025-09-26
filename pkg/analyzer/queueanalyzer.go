package analyzer

import (
	"fmt"

	"github.com/llm-inferno/queue-analysis/pkg/queue"

	utils "github.com/llm-inferno/queue-analysis/pkg/utils"
)

// create a new queue analyzer from config
func NewQueueAnalyzer(qConfig *Configuration, requestSize *RequestSize) (*QueueAnalyzer, error) {
	if err := qConfig.check(); err != nil {
		return nil, err
	}
	if err := requestSize.check(); err != nil {
		return nil, err
	}
	// build queueing model
	return BuildModel(qConfig, requestSize), nil
}

// build queueing model using service rates, leaving arrival rate as parameter
func BuildModel(qConfig *Configuration, requestSize *RequestSize) (modelData *QueueAnalyzer) {
	parms := qConfig.ServiceParms

	// calculate state-dependent service rate
	servRate := make([]float32, qConfig.MaxBatchSize)
	for n := 1; n <= qConfig.MaxBatchSize; n++ {
		prefillTime := parms.Prefill.PrefillTime(requestSize.AvgInputTokens, float32(n))
		decodeTime := float32(requestSize.AvgOutputTokens-1) * parms.Decode.DecodeTime(float32(n))
		servRate[n-1] = float32(n) / (prefillTime + decodeTime)
	}

	// set and check limits
	lambdaMin := servRate[0] * Epsilon
	lambdaMax := servRate[qConfig.MaxBatchSize-1] * (1 - Epsilon)
	rateRange := &RateRange{Min: lambdaMin * 1000, Max: lambdaMax * 1000}

	// create and solve model
	occupancyUpperBound := qConfig.MaxQueueSize + qConfig.MaxBatchSize
	model := queue.NewMM1ModelStateDependent(occupancyUpperBound, servRate)
	return &QueueAnalyzer{
		MaxBatchSize: qConfig.MaxBatchSize,
		MaxQueueSize: qConfig.MaxQueueSize,
		ServiceParms: parms,
		RequestSize:  requestSize,
		Model:        model,
		RateRange:    rateRange,
	}
}

// evaluate performance metrics given request rate
func (qa *QueueAnalyzer) Analyze(requestRate float32) (metrics *AnalysisMetrics, err error) {
	if requestRate <= 0 {
		return nil, fmt.Errorf("invalid request rate %v", requestRate)
	}
	model := qa.Model
	rateRange := qa.RateRange
	if requestRate > rateRange.Max {
		err = fmt.Errorf("rate=%v, max allowed rate=%v", requestRate, rateRange.Max)
		return nil, err
	}

	//solve model
	model.Solve(requestRate/1000, 1)
	if !model.IsValid() {
		err = fmt.Errorf("invalid model %s", model)
		return nil, err
	}

	// get statistics
	avgNumInServ := model.GetAvgNumInServers()

	effConc := EffectiveConcurrency(model.GetAvgServTime(), qa.ServiceParms, qa.RequestSize, qa.MaxBatchSize)
	prefillTime := qa.ServiceParms.Prefill.PrefillTime(qa.RequestSize.AvgInputTokens, effConc)
	tokenTime := qa.ServiceParms.Decode.DecodeTime(effConc)

	rho := avgNumInServ / float32(qa.MaxBatchSize)
	rho = min(max(rho, 0), 1)

	// return solution
	metrics = &AnalysisMetrics{
		Throughput:     model.GetThroughput() * 1000,
		AvgRespTime:    model.GetAvgRespTime(),
		AvgWaitTime:    model.GetAvgWaitTime(),
		AvgNumInServ:   avgNumInServ,
		AvgPrefillTime: prefillTime,
		AvgTokenTime:   tokenTime,
		MaxRate:        rateRange.Max,
		Rho:            rho,
	}
	return metrics, nil
}

// global variables used by eval functions, to be set before calling eval function
var evalRequestSize *RequestSize   // number of input and output tokens per request
var evalServiceParms *ServiceParms // request processing parameters for prefill and decode stages
var evalMaxBatchSize int           // max batch size

// evaluate max request rates to achieve a given target performance, returns
//   - max request rates
//   - performance metrics at min of max request rates
//   - achieved values of targets
func (qa *QueueAnalyzer) Size(targetPerf *TargetPerf) (targetRate *TargetRate, metrics *AnalysisMetrics, achieved *TargetPerf, err error) {
	if err := targetPerf.check(); err != nil {
		return nil, nil, nil, err
	}
	targetTTFT := targetPerf.TargetTTFT
	targetITL := targetPerf.TargetITL
	targetTPS := targetPerf.TargetTPS

	lambdaMin := qa.RateRange.Min / 1000
	lambdaMax := qa.RateRange.Max / 1000

	// set global variables for model and parameters used in functional evaluation
	utils.Model = qa.Model
	evalRequestSize = qa.RequestSize
	evalServiceParms = qa.ServiceParms
	evalMaxBatchSize = qa.MaxBatchSize

	var ind int

	// find max rate to achieve target TTFT time
	lambdaStarTTFT := lambdaMax
	if targetTTFT > 0 {
		lambdaStarTTFT, ind, err = utils.BinarySearch(lambdaMin, lambdaMax, targetTTFT, EvalTTFT)
		if ind < 0 {
			err = fmt.Errorf("target is below the bounded region")
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to calculate lambdaStarTTFT, targetTTFT=%v, range=%s, ind=%d, err=%v",
				targetTTFT, qa.RateRange, ind, err)
		}
	}

	// find max rate to achieve target ITL time
	lambdaStarITL := lambdaMax
	if targetITL > 0 {
		lambdaStarITL, ind, err = utils.BinarySearch(lambdaMin, lambdaMax, targetITL, EvalITL)
		if ind < 0 {
			err = fmt.Errorf("target is below the bounded region")
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to calculate lambdaStarITL, targetITL=%v, range=%s, ind=%d, err=%v",
				targetITL, qa.RateRange, ind, err)
		}
	}

	// find max rate to achieve target TPS
	lambdaStarTPS := lambdaMax
	if targetTPS > 0 {
		lambdaStarTPS = lambdaMax * (1 - StabilitySafetyFraction)
	}

	// analyze queue with smaller of rates
	lambda := min(lambdaStarTTFT, lambdaStarITL, lambdaStarTPS)
	requestRate := lambda * 1000 // convert to per-second rate
	if metrics, err = qa.Analyze(requestRate); err != nil {
		return nil, nil, nil, err
	}

	targetRate = &TargetRate{
		RateTargetTTFT: lambdaStarTTFT * 1000,
		RateTargetITL:  lambdaStarITL * 1000,
		RateTargetTPS:  lambdaStarTPS * 1000,
	}

	achieved = &TargetPerf{
		TargetTTFT: metrics.AvgWaitTime + metrics.AvgPrefillTime,
		TargetITL:  metrics.AvgTokenTime,
		TargetTPS:  metrics.Throughput * float32(qa.RequestSize.AvgOutputTokens),
	}
	return targetRate, metrics, achieved, nil
}

func (p *PrefillParms) PrefillTime(avgInputTokens int, batchSize float32) float32 {
	if avgInputTokens == 0 {
		return 0
	}
	return p.Gamma + p.Delta*float32(avgInputTokens)*batchSize
}

func (p *DecodeParms) DecodeTime(batchSize float32) float32 {
	return p.Alpha + p.Beta*batchSize
}

// Function used in binary search (target TTFT)
//   - x is lambda req/msec
func EvalTTFT(x float32) (float32, error) {
	utils.Model.Solve(x, 1)
	if !utils.Model.IsValid() {
		return 0, fmt.Errorf("invalid model %s", utils.Model)
	}
	avgWaitTime := utils.Model.GetAvgWaitTime()
	effConc := EffectiveConcurrency(utils.Model.GetAvgServTime(), evalServiceParms, evalRequestSize, evalMaxBatchSize)
	ttft := avgWaitTime + evalServiceParms.Prefill.PrefillTime(evalRequestSize.AvgInputTokens, effConc)
	return ttft, nil
}

// Function used in binary search (target ITL)
//   - x is lambda req/msec
func EvalITL(x float32) (float32, error) {
	utils.Model.Solve(x, 1)
	if !utils.Model.IsValid() {
		return 0, fmt.Errorf("invalid model %s", utils.Model)
	}
	effConc := EffectiveConcurrency(utils.Model.GetAvgServTime(), evalServiceParms, evalRequestSize, evalMaxBatchSize)
	return evalServiceParms.Decode.DecodeTime(effConc), nil
}

// calculate effective average number of requests in service (n), given average request service time
//   - n has to satisfy: prefillTime(n) + totalDecodeTime(n) = avgServiceTime
//   - prefillTime(n) = gamma + delta * inTokens * n
//   - totalDecodeTime(n) = (alpha + beta * n) * (outTokens - 1)
func EffectiveConcurrency(avgServiceTime float32, serviceParms *ServiceParms, requestSize *RequestSize, maxBatchSize int) float32 {
	tokens := float32(requestSize.AvgOutputTokens - 1)
	numerator := avgServiceTime - (serviceParms.Prefill.Gamma + serviceParms.Decode.Alpha*tokens)
	denominator := (serviceParms.Prefill.Delta * float32(requestSize.AvgInputTokens)) + (serviceParms.Decode.Beta * tokens)
	n := numerator / denominator
	return min(max(n, 0), float32(maxBatchSize))
}

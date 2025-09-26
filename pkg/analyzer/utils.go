package analyzer

import "fmt"

// check validity of configuration parameters
func (c *Configuration) check() error {
	if c.MaxBatchSize <= 0 || c.MaxQueueSize < 0 || c.ServiceParms == nil ||
		c.ServiceParms.Prefill == nil || c.ServiceParms.Decode == nil {
		return fmt.Errorf("invalid configuration %s", c)
	}
	return nil
}

// check validity of request size
func (rq *RequestSize) check() error {
	if rq.AvgInputTokens < 0 || rq.AvgOutputTokens < 1 {
		return fmt.Errorf("invalid request size %s", rq)
	}
	return nil
}

// check validity of target values
func (targetPerf *TargetPerf) check() error {
	if targetPerf.TargetITL < 0 ||
		targetPerf.TargetTTFT < 0 ||
		targetPerf.TargetTPS < 0 {
		return fmt.Errorf("invalid target data values %s", targetPerf)
	}
	return nil
}

/*
 * toString() functions
 */

func (c *Configuration) String() string {
	return fmt.Sprintf("{maxBatch=%d, maxQueue=%d, servParms:%s}",
		c.MaxBatchSize, c.MaxQueueSize, c.ServiceParms)
}

func (qa *QueueAnalyzer) String() string {
	return fmt.Sprintf("{maxBatch=%d, maxQueue=%d, servParms:%s, reqSize:%s, model:%s, rates:%s}",
		qa.MaxBatchSize, qa.MaxQueueSize, qa.ServiceParms, qa.RequestSize, qa.Model, qa.RateRange)
}

func (sp *ServiceParms) String() string {
	return fmt.Sprintf("{prefillParms=%s, decodeParms=%s}",
		sp.Prefill, sp.Decode)
}

func (p *PrefillParms) String() string {
	return fmt.Sprintf("{gamma=%.3f, delta=%.5f}", p.Gamma, p.Delta)
}

func (p *DecodeParms) String() string {
	return fmt.Sprintf("{alpha=%.3f, beta=%.5f}", p.Alpha, p.Beta)
}

func (rq *RequestSize) String() string {
	return fmt.Sprintf("{inTokens=%d, outTokens=%d}", rq.AvgInputTokens, rq.AvgOutputTokens)
}

func (rr *RateRange) String() string {
	return fmt.Sprintf("[%.3f, %.3f]", rr.Min, rr.Max)
}

func (am *AnalysisMetrics) String() string {
	return fmt.Sprintf("{tput=%.3f, lat=%.3f, wait=%.3f, conc=%.3f, prefill=%.3f, itl=%.3f, maxRate=%.3f, rho=%0.3f}",
		am.Throughput, am.AvgRespTime, am.AvgWaitTime, am.AvgNumInServ, am.AvgPrefillTime, am.AvgTokenTime, am.MaxRate, am.Rho)
}

func (tp *TargetPerf) String() string {
	return fmt.Sprintf("{TTFT=%.3f, ITL=%.3f, TPS=%.3f}",
		tp.TargetTTFT, tp.TargetITL, tp.TargetTPS)
}

func (tr *TargetRate) String() string {
	return fmt.Sprintf("{rateTTFT=%.3f, rateITL=%.3f, rateTPS=%.3f}",
		tr.RateTargetTTFT, tr.RateTargetITL, tr.RateTargetTPS)
}

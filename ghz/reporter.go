package ghz

import (
	"log"
	"sort"
	"time"

	// To register the xds resolvers and balancers.
	_ "google.golang.org/grpc/xds"
)

// Max size of the buffer of result channel.
const maxResult = 1000000

var ResultChannel chan *CallResult

func init() {
	ResultChannel = make(chan *CallResult, min(1000, maxResult))
}

func NewReporter(name string, skipFirst int, countErrorLatency bool) *Reporter {
	return &Reporter{
		results:        ResultChannel,
		done:           make(chan bool, 1),
		details:        make([]ResultDetail, 0, maxResult),
		statusCodeDist: make(map[string]int),
		errorDist:      make(map[string]int),
		stopReason:     ReasonNormalEnd.String(),
		config:         &RunConfig{Name: name, SkipFirst: skipFirst, CountErrorLatency: countErrorLatency},
	}
}

// Kicks off the report tracking
func (r *Reporter) Run() {
	r.start = time.Now()
	go func() {
		r.Track()
	}()
}

// Run runs the reporter
func (r *Reporter) Track() {
	var skipCount int

	for res := range r.results {
		if skipCount < r.config.SkipFirst {
			skipCount++
			continue
		}

		errStr := ""
		r.totalCount++
		r.totalLatenciesSec += res.Duration.Seconds()
		r.statusCodeDist[res.Status]++

		if res.Err != nil {
			errStr = res.Err.Error()
			r.errorDist[errStr]++
		}

		if len(r.details) < maxResult {
			r.details = append(r.details, ResultDetail{
				Latency:   res.Duration,
				Timestamp: res.Timestamp,
				Status:    res.Status,
				Error:     errStr,
			})
		}
	}
	r.done <- true
}

func (r *Reporter) Stop(reason StopReason) *Report {
	r.stopReason = reason.String()
	log.Printf("Stopping with reason: %+v", reason)

	close(r.results)
	total := time.Since(r.start)
	log.Printf("Waiting for report")

	// Wait until the reporter is done.
	<-r.done

	log.Printf("Finalizing report")
	return r.Finalize(reason, total)
}

// Finalize all the gathered data into a final report
func (r *Reporter) Finalize(stopReason StopReason, total time.Duration) *Report {
	rep := &Report{
		Name:           r.config.Name,
		EndReason:      stopReason,
		Date:           time.Now(),
		Count:          r.totalCount,
		ErrorDist:      r.errorDist,
		StatusCodeDist: r.statusCodeDist}

	if len(r.details) > 0 {
		average := r.totalLatenciesSec / float64(r.totalCount)
		rep.Average = time.Duration(average * float64(time.Second))

		rep.Rps = float64(r.totalCount) / total.Seconds()

		okLats := make([]float64, 0)
		for _, d := range r.details {
			if d.Error == "" || r.config.CountErrorLatency {
				okLats = append(okLats, d.Latency.Seconds())
			}
		}
		sort.Float64s(okLats)
		if len(okLats) > 0 {
			var fastestNum, slowestNum float64
			fastestNum = okLats[0]
			slowestNum = okLats[len(okLats)-1]

			rep.Fastest = time.Duration(fastestNum * float64(time.Second))
			rep.Slowest = time.Duration(slowestNum * float64(time.Second))
			rep.Histogram = trackHistogram(okLats, slowestNum, fastestNum)
			rep.LatencyDistribution = latencies(okLats)
		}

		rep.Details = r.details
	}

	return rep
}

func trackHistogram(latencies []float64, slowest, fastest float64) []Bucket {
	bc := 10
	buckets := make([]float64, bc+1)
	counts := make([]int, bc+1)
	bs := (slowest - fastest) / float64(bc)
	for i := 0; i < bc; i++ {
		buckets[i] = fastest + bs*float64(i)
	}
	buckets[bc] = slowest
	var bi int
	var max int
	for i := 0; i < len(latencies); {
		if latencies[i] <= buckets[bi] {
			i++
			counts[bi]++
			if max < counts[bi] {
				max = counts[bi]
			}
		} else if bi < len(buckets)-1 {
			bi++
		}
	}
	res := make([]Bucket, len(buckets))
	for i := 0; i < len(buckets); i++ {
		res[i] = Bucket{
			Mark:      buckets[i],
			Count:     counts[i],
			Frequency: float64(counts[i]) / float64(len(latencies)),
		}
	}
	return res
}

func latencies(latencies []float64) []LatencyDistribution {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	data := make([]float64, len(pctls))
	lt := float64(len(latencies))
	for i, p := range pctls {
		ip := (float64(p) / 100.0) * lt
		di := int(ip)

		// since we're dealing with 0th based ranks we need to
		// check if ordinal is a whole number that lands on the percentile
		// if so adjust accordingly
		if ip == float64(di) {
			di = di - 1
		}

		if di < 0 {
			di = 0
		}

		data[i] = latencies[di]
	}

	res := make([]LatencyDistribution, len(pctls))
	for i := 0; i < len(pctls); i++ {
		if data[i] > 0 {
			lat := time.Duration(data[i] * float64(time.Second))
			res[i] = LatencyDistribution{Percentage: pctls[i], Latency: lat}
		}
	}
	return res
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
